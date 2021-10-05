# aws-dynamic-prefix-list

Example (sanitized) Golang Lambda function to retrieve allowed CIDR ranges from two API endpoints and modify an AWS Prefix List. This Prefix List can then be used within security groups to manage access to resources dynamically.

CI Pipeline should execute the build/compile:

```
go mod init whatever
go mod tidy
GOOS=linux GOARCH=amd64 go build -o main && chmod 777 main && zip function.zip main
```

And then can be deployed with Terraform, e.g.:

```
resource "aws_lambda_function" "this" {
  function_name    = "autoPrefixList"
  filename         = "${path.module}/function.zip"
  handler          = "main"
  runtime          = "go1.x"
  role             = aws_iam_role.this.arn
  timeout          = 30

  environment {
    variables = {
      API_TOKEN = var.api_token
    }
  }
}

resource "aws_lambda_permission" "this" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.this.arn
}

resource "aws_iam_role" "this" {
  name = "this-iam-role"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": [
          "lambda.amazonaws.com"
        ]
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "this" {
  name   = "this-iam-policy"
  role   = aws_iam_role.this.id
  policy = <<EOF
{
  "Version": "2012-10-17",	
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeNetworkInterfaces",
        "ec2:CreateNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:DescribeManagedPrefixLists",
        "ec2:RestoreManagedPrefixListVersion",
        "ec2:CreateManagedPrefixList",
        "ec2:ModifyManagedPrefixList",
        "ec2:GetManagedPrefixListEntries"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
        "logs:PutLogEvents",
        "logs:GetLogEvents",
        "logs:FilterLogEvents"
      ],
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_cloudwatch_event_rule" "this" {
  name                = "this-event-rule"
  schedule_expression = "cron(0/5 * * * ? *)"
}

resource "aws_cloudwatch_event_target" "this" {
  rule = aws_cloudwatch_event_rule.this.name
  arn  = aws_lambda_function.this.arn
}
```
