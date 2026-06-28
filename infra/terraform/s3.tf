resource "aws_s3_bucket" "profile" {
  bucket        = var.s3_bucket_profile
  force_destroy = true

  tags = { Name = var.s3_bucket_profile }
}

resource "aws_s3_bucket" "matches" {
  bucket        = var.s3_bucket_matches
  force_destroy = true

  tags = { Name = var.s3_bucket_matches }
}

resource "aws_s3_bucket" "business_docs" {
  bucket        = var.s3_bucket_business_docs
  force_destroy = true

  tags = { Name = var.s3_bucket_business_docs }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "business_docs" {
  bucket = aws_s3_bucket.business_docs.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "business_docs" {
  bucket                  = aws_s3_bucket.business_docs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
