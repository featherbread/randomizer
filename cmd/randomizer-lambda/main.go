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

	"github.com/aws-observability/aws-otel-go/exporters/xrayudp"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	lambdadetector "go.opentelemetry.io/contrib/detectors/aws/lambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda/xrayconfig"
	xraypropagator "go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/featherbread/randomizer/internal/slack"
	"github.com/featherbread/randomizer/internal/store/dynamodb"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	tokenProvider, err := slack.TokenProviderFromEnv()
	if err != nil {
		logger.Error("Failed to configure Slack token", "err", err)
		os.Exit(2)
	}

	storeFactory, err := dynamodb.FactoryFromEnv(ctx)
	if err != nil {
		logger.Error("Failed to create DynamoDB store", "err", err)
		os.Exit(2)
	}

	// OpenTelemetry is always active, but traces are only exported to AWS X-Ray
	// (and charged for usage) if enabled in the environment.
	tp := initTracerProvider(ctx, logger)
	otel.SetTracerProvider(tp)
	if xrayTracingEnabled {
		otel.SetTextMapPropagator(xraypropagator.Propagator{})
	}
	defer func() {
		err := tp.Shutdown(ctx)
		if err != nil {
			logger.Warn("Failed to shut down tracer provider", "err", err)
		}
	}()

	app := slack.App{
		TokenProvider: tokenProvider,
		StoreFactory:  storeFactory,
		Logger:        logger,
	}
	appHandler := httpadapter.NewV2(app).ProxyWithContext
	handler := otellambda.InstrumentHandler(appHandler, xrayconfig.WithRecommendedOptions(tp)...)
	lambda.Start(handler)
}

var xrayTracingEnabled = os.Getenv("AWS_XRAY_TRACING_ENABLED") == "1"

func initTracerProvider(ctx context.Context, logger *slog.Logger) *trace.TracerProvider {
	traceResource := initTraceResource(ctx, logger)
	tp := trace.NewTracerProvider(trace.WithResource(traceResource))

	if !xrayTracingEnabled {
		return tp
	}

	exporter, err := xrayudp.NewSpanExporter(ctx)
	if err != nil {
		logger.Warn("Failed to initialize X-Ray span exporter", "err", err)
		return tp
	}

	tp.RegisterSpanProcessor(trace.NewSimpleSpanProcessor(exporter))
	return tp
}

func initTraceResource(ctx context.Context, logger *slog.Logger) *resource.Resource {
	baseResource := resource.NewWithAttributes(semconv.SchemaURL, attribute.KeyValue{
		Key:   semconv.ServiceNameKey,
		Value: attribute.StringValue(os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))})

	if !xrayTracingEnabled {
		return baseResource
	}

	lambdaResource, err := lambdadetector.NewResourceDetector().Detect(ctx)
	if err != nil {
		logger.Warn("Skipping Lambda resources in traces", "err", err, "step", "detect")
		return baseResource
	}

	mergedResource, err := resource.Merge(lambdaResource, baseResource)
	if err != nil {
		logger.Warn("Skipping Lambda resources in traces", "err", err, "step", "merge")
		return baseResource
	}

	return mergedResource
}
