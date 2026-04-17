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
