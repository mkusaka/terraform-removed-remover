
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}


removed {
  from = aws_instance.actual_old
  lifecycle {
    destroy = false
  }
}
