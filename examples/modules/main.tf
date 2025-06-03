module "networking" {
  source = "./networking"
  
  vpc_cidr = "10.0.0.0/16"
}

removed {
  from = module.old_networking
  lifecycle {
    destroy = false
  }
}
