package main

import (
	"./handler"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler.HandleRequest)
}
