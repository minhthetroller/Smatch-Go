# AWS Load Testing Runbook

This repo no longer manages AWS Fault Injection Service experiments. Use AWS
Distributed Load Testing (DLT) for request-driven backend CPU tests.

## Backend Stress Endpoint

The backend exposes a protected stress endpoint for controlled load tests:

```text
POST /api/load-test/stress?duration_ms=750&workers=1
X-Admin-Secret: <ADMIN_SECRET>
```

The endpoint is disabled by default. Enable it only while running DLT:

```hcl
load_test_stress_enabled = true
admin_secret             = "CHANGE_ME"
```

Bounds:

- `duration_ms`: default `750`, maximum `5000`.
- `workers`: default `1`, maximum `runtime.NumCPU()` on the instance.

The endpoint performs CPU-bound math work and returns elapsed time plus a
checksum. It performs no database, Redis, S3, or large-memory work.

## Deploy Backend Changes

Build and push both images because request logging is shared by both servers:

```bash
docker build --build-arg SERVICE=server -t smatch-server .
docker build --build-arg SERVICE=admin-server -t smatch-admin-server .
```

Tag and push these images to the ECR repository referenced by `ecr_repo_url`.
The backend ASG expects tag `latest`; the admin ASG expects tag `admin`.

Run Terraform:

```bash
terraform -chdir=infra/terraform init
terraform -chdir=infra/terraform fmt
terraform -chdir=infra/terraform validate
terraform -chdir=infra/terraform plan -out=tfplan.out
terraform -chdir=infra/terraform apply tfplan.out
```

Do not commit `infra/terraform/terraform.tfvars` or `tfplan.out`.

## Verify With AWS CLI

Capture Terraform outputs:

```bash
API_URL=$(terraform -chdir=infra/terraform output -raw api_url)
BACKEND_ASG=$(terraform -chdir=infra/terraform output -raw asg_name)
ADMIN_ASG=$(terraform -chdir=infra/terraform output -raw admin_asg_name)
BACKEND_TG=$(terraform -chdir=infra/terraform output -raw backend_target_group_arn)
BACKEND_LOG_GROUP=$(terraform -chdir=infra/terraform output -raw backend_log_group)
ADMIN_LOG_GROUP=$(terraform -chdir=infra/terraform output -raw admin_log_group)
LAMBDA_NAME=$(terraform -chdir=infra/terraform output -raw log_alarm_notifier_function_name)
```

Verify deployment health:

```bash
aws autoscaling describe-auto-scaling-groups \
  --auto-scaling-group-names "$BACKEND_ASG" "$ADMIN_ASG" \
  --query 'AutoScalingGroups[].{name:AutoScalingGroupName,desired:DesiredCapacity,inService:length(Instances[?LifecycleState==`InService`])}'

aws elbv2 describe-target-health \
  --target-group-arn "$BACKEND_TG" \
  --query 'TargetHealthDescriptions[].{target:Target.Id,state:TargetHealth.State,reason:TargetHealth.Reason}'

aws lambda get-function \
  --function-name "$LAMBDA_NAME" \
  --query 'Configuration.{name:FunctionName,state:State,lastModified:LastModified}'
```

Verify alarms and reduced request logs:

```bash
aws cloudwatch describe-alarms \
  --alarm-names \
  "$(terraform -chdir=infra/terraform output -raw backend_cpu_alarm_name)" \
  "$(terraform -chdir=infra/terraform output -raw admin_cpu_alarm_name)" \
  --query 'MetricAlarms[].{name:AlarmName,state:StateValue,threshold:Threshold}'

aws logs filter-log-events \
  --log-group-name "$BACKEND_LOG_GROUP" \
  --filter-pattern '{ $.msg = "http request completed" }' \
  --max-items 10

aws logs filter-log-events \
  --log-group-name "$ADMIN_LOG_GROUP" \
  --filter-pattern '{ $.msg = "http request completed" }' \
  --max-items 10
```

Only WARN/ERROR request logs should appear. Normal fast 2xx/3xx requests should
not be present.

Verify the stress endpoint before DLT:

```bash
curl -sS -i \
  -X POST \
  -H "X-Admin-Secret: ${ADMIN_SECRET}" \
  "${API_URL}/api/load-test/stress?duration_ms=750&workers=1"
```

## Launch AWS Distributed Load Testing

Use AWS Launch Wizard for AWS Distributed Load Testing in `ap-southeast-1`.

Use these values:

- Console administrator name: `smatchadmin`
- Console administrator email: `nguyentuanminh1105@gmail.com`
- Target URL: `${API_URL}/api/load-test/stress?duration_ms=750&workers=1`
- Method: `POST`
- Header: `X-Admin-Secret: <ADMIN_SECRET>`

After Launch Wizard completes, verify the DLT stack:

```bash
aws cloudformation describe-stacks \
  --stack-name <dlt-stack-name> \
  --query 'Stacks[0].{name:StackName,status:StackStatus,outputs:Outputs}'
```

Run the DLT test from the DLT web console. Keep `duration_ms` and `workers`
conservative for the first run, then increase DLT concurrency gradually.

## CloudFront Free Flat-Rate Plan

The admin web CloudFront distribution remains managed by Terraform, but the
CloudFront Free flat-rate pricing plan is a console-managed account/distribution
setting.

Use the CloudFront console to move the distribution from pay-as-you-go to the
Free flat-rate plan if the account and distribution are eligible:

```bash
terraform -chdir=infra/terraform output -raw web_cloudfront_distribution_id
```

Current local AWS CLI support does not expose `pricingplanmanager`, so verify
the pricing plan in the CloudFront console. If AWS blocks enrollment, keep
pay-as-you-go and record the blocker before applying future CloudFront changes.
