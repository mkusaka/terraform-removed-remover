provider "aws" {
  region = "us-west-2"
}

resource "aws_instance" "web_server" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
  
  tags = {
    Name = "WebServer"
  }
}

removed {
  from = aws_instance.old_web_server
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-data-bucket"
}

removed {
  from = aws_s3_bucket.old_logs
  lifecycle {
    destroy = true
  }
}
