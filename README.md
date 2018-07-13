# ebs backup golang

「Backup-Generation」タグがついたインスタンスのEBSバックアップを取得します。  
ステータスがrunningのもののみが対象となります。

## Usage

go build
```
GOOS=linux GOARCH=amd64 go build -o build/ebs-backup
```

convert template file
```
aws cloudformation package --template-file template.yml \
  --s3-bucket [your s3 bucket name] \
  --s3-prefix ebs-backup \
  --output-template-file .template.yml
```

deploy
```
aws cloudformation deploy \
  --template-file .template.yml \
  --stack-name [your stack name] \
  --capabilities CAPABILITY_IAM
```
