# Randomizer: AWS Lambda Deployment

The randomizer supports deployment to AWS Lambda, allowing you to set it up
without the need to manage servers or other infrastructure. This directory
includes tools and instructions that will help you perform the deployment.

If you don't already have an AWS account, sign up at https://aws.amazon.com/ to
get started.

## Install and Configure Required Tools

In addition to a working Go installation, the deployment script requires the
[AWS CLI][install-aws-cli]. Versions 1 and 2 should both work. If you happen to
be using [Homebrew][brew], you can install the AWS CLI with a single command:

```sh
brew install awscli
```

After installing the AWS CLI, see [Configuring the AWS CLI][configure] to set up
access to your AWS account. This requires a set of credentials from AWS; the
guide explains how to obtain these if you're not already familiar with [AWS
IAM][iam].

(TODO: Discuss what IAM policies the CLI user needs to have.)

[install-aws-cli]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[brew]: https://brew.sh
[configure]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[iam]: https://aws.amazon.com/iam/

## Create an S3 Bucket

When AWS Lambda starts up your function, it downloads the compiled randomizer
code from an [Amazon S3][s3] bucket. You can create a new S3 bucket using the
AWS CLI:

```sh
aws s3 mb s3://[name]
```

You can choose whatever `[name]` you'd like, but keep in mind that S3 bucket
names must be unique across **all AWS accounts**. For example, the name
`randomizer` has already been taken by some unknown AWS user. You'll probably
want a name that references yourself, your company, etc.

[s3]: https://aws.amazon.com/s3/

## Add the Slack Verification Token to the AWS SSM Parameter Store

The randomizer validates that each HTTP request legitimately came from Slack by
checking for a special Slack-provided token value in the request parameters.
Since this token is a secret value, you should store it in the [AWS Systems
Manager Parameter Store][ssm parameter store] with encryption.

Note that the current version of the randomizer only supports the deprecated
"Verification Token" to validate requests, and not the newer "Signing Secret"
configuration.

The token value is available on the "Basic Information" page of the Slack app
configuration interface. Once you have it, you can create the parameter using
the AWS CLI:

```sh
aws ssm put-parameter --type SecureString --name /Randomizer/SlackToken --value <token>
```

The parameter name in the `aws ssm` command is unique within your AWS account,
must start with a `/`, and can contain extra slash-separated parts to help
organize all the SSM parameters in your account. While you can encrypt the
parameter with the default AWS-managed SSM key, the CloudFormation template
doesn't support encryption with a custom KMS key (which costs $1/mo and
requires extra IAM and KMS setup).

[ssm parameter store]: https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html

## Run the Initial Deployment

Now, you'll use AWS [CloudFormation][CloudFormation] to deploy the randomizer
into your account, with all necessary resources (like the DynamoDB table for
storing groups) automatically created and configured.

Similar to how you picked S3 bucket and SSM parameter names, you'll also need
to pick a name for your CloudFormation "stack." Like your repository name, this
needs to be unique within your AWS account. If you only need to deploy one copy
of the randomizer, a simple name like "Randomizer" should be enough.

To configure your deployment, create a `hfc.local.toml` file at the root of the
randomizer repository with values matching all of your previous decisions:

```toml
[upload]
# The name of the S3 bucket for Lambda code uploads.
bucket = "..."

[[stacks]]
# Whatever name you'd like. You can have multiple [[stacks]] if you need.
name = "Randomizer"
# The --name you created the SSM parameter with, without the leading slash.
parameters = { SlackTokenSSMName = "Randomizer/SlackToken" }
```

With your local configuration ready, run the helper script to start the
deployment:

```sh
./hfc build-deploy Randomizer  # or whatever other stack name you chose
```

This command automatically compiles the randomizer code for AWS Lambda, uploads
it to your S3 bucket, sets it up for use, and prints a webhook URL for Slack.
Copy and paste this into the "URL" field of your Slack slash command
configuration, and save it.

At this point, you should be able to use the randomizer in your Slack
workspace. Go ahead and try it out!

[CloudFormation]: https://aws.amazon.com/cloudformation/

## Upgrades and Maintenance

To upgrade the randomizer deployment in your AWS account, run
`./hfc build-deploy Randomizer` in a newer version of the repository.

Run `./hfc help` to learn more about additional commands that might be useful.

## Notes

- The CloudFormation template (Template.yaml) uses the [AWS SAM][sam]
  transformation to simplify the setup of the Lambda function.
- The template provisions the DynamoDB table in On-Demand capacity mode, which
  isn't eligible for the AWS Free Tier. See the [Read/Write Capacity
  Mode][capacity mode] documentation for details.
- The default configuration enables [AWS X-Ray][x-ray] tracing for the function
  and its AWS SDK requests. Every AWS account can collect up to 100,000 traces
  per month for free, which is useful to see where requests are spending time.
  However, you can turn this off by passing `XRayTracingEnabled=false` to the
  deployment script.
- My co-workers and I collectively make a little over 500 requests to the
  randomizer per month, and at that small of a volume it's essentially free to
  run on AWS even without the 12 month free tier. My _rough_ estimate is that
  the randomizer costs less than $1 per million requests.

[sam]: https://github.com/awslabs/serverless-application-model
[capacity mode]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.ReadWriteCapacityMode.html
[x-ray]: https://aws.amazon.com/xray/
