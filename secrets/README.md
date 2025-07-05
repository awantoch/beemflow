# BeemFlow Secrets Management

BeemFlow provides a clean, pluggable secrets management system that supports multiple backends while maintaining backward compatibility.

## Quick Start

### Environment Variables (Default)

No configuration needed - BeemFlow defaults to environment variables:

```bash
export API_KEY="your-secret-key"
export DB_PASSWORD="your-db-password"
```

Use in your flows:
```yaml
steps:
  - id: api_call
    use: http.post
    with:
      url: "https://api.example.com/data"
      headers:
        Authorization: "Bearer {{ secrets.API_KEY }}"
```

### AWS Secrets Manager

Configure in `flow.config.json`:
```json
{
  "secrets": {
    "driver": "aws-sm",
    "region": "us-west-2",
    "prefix": "beemflow/"
  }
}
```

This will look for secrets like `beemflow/API_KEY` in AWS Secrets Manager.

## Supported Drivers

- `env` - Environment variables (default)
- `aws-sm` - AWS Secrets Manager

## Configuration

### Environment Variables

```json
{
  "secrets": {
    "driver": "env",
    "prefix": "BEEMFLOW_"
  }
}
```

- `prefix`: Optional prefix for environment variables

### AWS Secrets Manager

```json
{
  "secrets": {
    "driver": "aws-sm", 
    "region": "us-west-2",
    "prefix": "beemflow/"
  }
}
```

- `region`: AWS region (required)
- `prefix`: Optional prefix for secret names

## Template Usage

In your YAML flows, use the `{{ secrets.KEY }}` syntax:

```yaml
name: example-flow
steps:
  - id: database
    use: postgres.query
    with:
      connection_string: "postgres://user:{{ secrets.DB_PASSWORD }}@localhost/db"
      query: "SELECT * FROM users"
      
  - id: api_call
    use: http.post
    with:
      url: "{{ secrets.API_ENDPOINT }}"
      headers:
        Authorization: "Bearer {{ secrets.API_TOKEN }}"
```

## Backward Compatibility

The system maintains full backward compatibility:

- Existing `{{ secrets.KEY }}` syntax works unchanged
- Environment variables with `$env:` prefix still work
- Secrets passed in events take precedence

## AWS Setup

For AWS Secrets Manager, ensure your environment has proper credentials:

1. **IAM Role** (recommended for EC2/ECS/Lambda):
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "secretsmanager:GetSecretValue"
         ],
         "Resource": "arn:aws:secretsmanager:*:*:secret:beemflow/*"
       }
     ]
   }
   ```

2. **Environment Variables**:
   ```bash
   export AWS_ACCESS_KEY_ID="your-access-key"
   export AWS_SECRET_ACCESS_KEY="your-secret-key"
   export AWS_REGION="us-west-2"
   ```

3. **AWS CLI Profile**:
   ```bash
   aws configure --profile beemflow
   export AWS_PROFILE=beemflow
   ```

## Examples

### Development Setup
```json
{
  "secrets": {
    "driver": "env"
  }
}
```

### Production Setup
```json
{
  "secrets": {
    "driver": "aws-sm",
    "region": "us-west-2", 
    "prefix": "myapp/prod/"
  }
}
```

This provides a clean upgrade path from development to production without changing your flows.