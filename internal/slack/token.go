package slack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.opentelemetry.io/otel/attribute"

	"github.com/featherbread/randomizer/internal/awsconfig"
)

const DefaultAWSParameterTTL = 2 * time.Minute

// TokenProvider provides the expected value of the slash command verification
// token that Slack includes in its requests.
type TokenProvider func(ctx context.Context) (string, error)

// TokenProviderFromEnv returns a TokenProvider based on available environment
// variables.
//
// If SLACK_TOKEN is set, it returns a static token provider.
//
// If SLACK_TOKEN_SSM_NAME is set, it returns an AWS SSM token provider,
// with the TTL optionally set by SLACK_TOKEN_SSM_TTL.
//
// Otherwise, it returns an error.
func TokenProviderFromEnv() (TokenProvider, error) {
	if token, ok := os.LookupEnv("SLACK_TOKEN"); ok {
		return StaticToken(token), nil
	}

	if ssmName, ok := os.LookupEnv("SLACK_TOKEN_SSM_NAME"); ok {
		ttl, err := ssmTTLFromEnv()
		if err != nil {
			return nil, err
		}
		return AWSParameter(ssmName, ttl), nil
	}

	return nil, errors.New("missing SLACK_TOKEN or SLACK_TOKEN_SSM_NAME in environment")
}

func ssmTTLFromEnv() (time.Duration, error) {
	ttlEnv, ok := os.LookupEnv("SLACK_TOKEN_SSM_TTL")
	if !ok {
		return DefaultAWSParameterTTL, nil
	}

	ttl, err := time.ParseDuration(ttlEnv)
	if err != nil {
		return 0, fmt.Errorf("SLACK_TOKEN_SSM_TTL is not a valid Go duration: %w", err)
	}

	return ttl, nil
}

// StaticToken uses token as the expected value of the verification token.
func StaticToken(token string) TokenProvider {
	return func(_ context.Context) (string, error) {
		return token, nil
	}
}

// AWSParameter retrieves the expected value of the verification token from the
// AWS SSM Parameter Store, decrypting it if necessary, and caches the retrieved
// token value for the provided TTL.
func AWSParameter(name string, ttl time.Duration) TokenProvider {
	var (
		lock   = make(chan struct{}, 1)
		token  string
		expiry time.Time
	)

	return func(ctx context.Context) (string, error) {
		ctx, span := tracer.Start(ctx, "slack.AWSParameter")
		defer span.End()

		select {
		case lock <- struct{}{}:
			defer func() {
				<-lock
				span.SetAttributes(
					attribute.Int64("randomizer.slack.ssm.expiry", expiry.Unix()))
			}()
		case <-ctx.Done():
			return "", ctx.Err()
		}

		if time.Now().Before(expiry) {
			span.SetAttributes(attribute.Bool("randomizer.slack.ssm.cached", true))
			return token, nil
		}

		span.SetAttributes(attribute.Bool("randomizer.slack.ssm.cached", false))
		cfg, err := awsconfig.New(ctx)
		if err != nil {
			return "", err
		}

		output, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(name),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return "", fmt.Errorf("loading Slack token parameter: %w", err)
		}

		token = *output.Parameter.Value
		expiry = time.Now().Add(ttl)
		return token, nil
	}
}
