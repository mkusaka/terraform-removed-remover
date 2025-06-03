removed {
  from = aws_instance.example
  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_s3_bucket.example
  lifecycle {
    destroy = true
  }
}
