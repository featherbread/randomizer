// The randomizer-lambda command is an AWS Lambda handler that serves the Slack
// slash command API for the randomizer.
//
// The handler expects HTTP request events using the [Amazon API Gateway
// payload format version 2.0]. This makes it suitable for invocation through a
// [Lambda function URL], or through an AWS Lambda proxy integration in an
// Amazon API Gateway HTTP API.
//
// See the randomizer repository README for more information on configuring and
// deploying the randomizer on AWS Lambda.
//
// [Amazon API Gateway payload format version 2.0]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
// [Lambda function URL]: https://docs.aws.amazon.com/lambda/latest/dg/lambda-urls.html
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda/xrayconfig"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/featherbread/randomizer/internal/slack"
	"github.com/featherbread/randomizer/internal/store/dynamodb"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	tokenProvider, err := slack.TokenProviderFromEnv()
	if err != nil {
		logger.Error("Failed to configure Slack token", "err", err)
		os.Exit(2)
	}

	storeFactory, err := dynamodb.FactoryFromEnv(context.Background())
	if err != nil {
		logger.Error("Failed to create DynamoDB store", "err", err)
		os.Exit(2)
	}

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	app := slack.App{
		TokenProvider: tokenProvider,
		StoreFactory:  storeFactory,
		Logger:        logger,
	}
	lambda.Start(
		otellambda.InstrumentHandler(
			httpadapter.NewV2(app).ProxyWithContext,
			xrayconfig.WithRecommendedOptions(tp)...))
}
