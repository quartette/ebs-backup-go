package handler

import (
	"log"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var sess = session.Must(session.NewSession())

// Snapshots 構造体のスライス
type Snapshots []*ec2.Snapshot

func (s Snapshots) Len() int {
	return len(s)
}

func (s Snapshots) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less 作成開始時間降順でソートさせる
func (s Snapshots) Less(i, j int) bool {
	return s[i].StartTime.Before(*s[j].StartTime)
}

// VolumInfo ボリューム情報
type VolumInfo struct {
	volumeID    string
	description string
}

// InstancesDescription インスタンス情報
type InstancesDescription struct {
	instanceID string
	volumes    []VolumInfo
	name       string
	generation string
}

// BackupInfo バックアップ対象のインスタンス情報の配列
type BackupInfo []InstancesDescription

func createSnapShotTag(ec2client *ec2.EC2, id string, volumeID string, name string) {
	log.Println("create snapshot tag snapshotId : " + id)
	params := &ec2.CreateTagsInput{
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String("Name"),
				Value: aws.String(name),
			},
			&ec2.Tag{
				Key:   aws.String("VolumeId"),
				Value: aws.String(volumeID),
			},
		},
		DryRun: aws.Bool(false),
	}
	_, err := ec2client.CreateTags(params)
	if err != nil {
		panic(err)
	}
}

func createSnapShot(ec2client *ec2.EC2, backupInfo *BackupInfo) {

	for _, info := range *backupInfo {
		for _, volume := range info.volumes {
			params := &ec2.CreateSnapshotInput{
				VolumeId:    aws.String(volume.volumeID),
				Description: aws.String(volume.description),
				DryRun:      aws.Bool(false),
			}
			log.Println("create snapshot volumeId : " + volume.volumeID)
			res, err := ec2client.CreateSnapshot(params)
			if err != nil {
				panic(err)
			}
			createSnapShotTag(ec2client, *res.SnapshotId, volume.volumeID, info.name)
		}

	}
}

func parseDescriptions(instance *ec2.Instance) (InstancesDescription, error) {
	var tagName string
	var generation string

	for _, t := range instance.Tags {
		if *t.Key == "Name" {
			tagName = *t.Value
		} else if *t.Key == "Backup-Generation" {
			generation = *t.Value
		}
	}
	if tagName == "" {
		tagName = *instance.InstanceId
	}
	var volumes []VolumInfo
	for _, b := range instance.BlockDeviceMappings {
		if b.Ebs != nil {
			volumes = append(volumes, VolumInfo{
				volumeID:    *b.Ebs.VolumeId,
				description: "Auto Snapshot " + tagName + " volumeId: " + *b.Ebs.VolumeId,
			})
		}
	}
	return InstancesDescription{
		instanceID: *instance.InstanceId,
		volumes:    volumes,
		name:       tagName,
		generation: generation,
	}, nil
}

func fetchSnapshotByVolumeID(ec2client *ec2.EC2, volumeID string) []*ec2.Snapshot {

	log.Println("fetch snapshot by volumeID : " + volumeID)
	params := &ec2.DescribeSnapshotsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("volume-id"),
				Values: []*string{aws.String(volumeID)},
			},
		},
	}
	res, err := ec2client.DescribeSnapshots(params)
	if err != nil {
		panic(err)
	}
	return res.Snapshots
}

func deleteSnapshot(ec2client *ec2.EC2, id string) {

	params := &ec2.DeleteSnapshotInput{
		SnapshotId: aws.String(id),
		DryRun:     aws.Bool(false),
	}
	ec2client.DeleteSnapshot(params)
}

func deleteOldSnapshot(ec2client *ec2.EC2, backupInfo *BackupInfo) {

	for _, info := range *backupInfo {
		generation, _ := strconv.Atoi(info.generation)
		for _, volume := range info.volumes {
			var snapshots Snapshots = fetchSnapshotByVolumeID(ec2client, volume.volumeID)

			if generation < len(snapshots) {
				sort.Sort(snapshots)
				deleteNum := len(snapshots) - generation
				for i, snapshot := range snapshots {
					if deleteNum > i {
						log.Println("delete snapshot : " + *snapshot.SnapshotId)
						deleteSnapshot(ec2client, *snapshot.SnapshotId)
					}
				}
			}
		}
	}
}

func fetchTargetInstances(ec2client *ec2.EC2) BackupInfo {
	log.Println("fetch target instances")
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String("Backup-Generation")},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
					// aws.String("stopped"),
				},
			},
		},
	}
	res, err := ec2client.DescribeInstances(params)
	if err != nil {
		panic(err)
	}

	var backupInfo BackupInfo
	for _, r := range res.Reservations {
		for _, i := range r.Instances {
			description, err := parseDescriptions(i)
			if err != nil {
				panic(err)
			}
			backupInfo = append(backupInfo, description)
		}
	}
	return backupInfo
}

// HandleRequest Lambdaから呼ばれるハンドラ
func HandleRequest() (string, error) {
	ec2client := ec2.New(sess)
	backupInfo := fetchTargetInstances(ec2client)
	createSnapShot(ec2client, &backupInfo)
	deleteOldSnapshot(ec2client, &backupInfo)
	return "ebs backup done", nil
}
