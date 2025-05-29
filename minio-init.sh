#!/bin/sh

# Wait for MinIO to be ready
echo "Waiting for MinIO to be ready..."
until mc alias set myminio http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"; do
  echo "MinIO is unavailable - sleeping"
  sleep 1
done

echo "MinIO is ready! Creating bucket and setting up policies..."

# Create the bucket if it doesn't exist
mc mb --ignore-existing myminio/"$MINIO_BUCKET_NAME"

# Create a new policy for the service user
cat > /tmp/policy.json << EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::${MINIO_BUCKET_NAME}/*",
                "arn:aws:s3:::${MINIO_BUCKET_NAME}"
            ]
        }
    ]
}
EOF

# Create a new policy
mc admin policy create myminio service-policy /tmp/policy.json

# Create service user if it doesn't exist
mc admin user add myminio "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" || true

# Assign the policy to the service user
mc admin policy attach myminio service-policy --user "$MINIO_ACCESS_KEY"

echo "MinIO initialization completed successfully!" 