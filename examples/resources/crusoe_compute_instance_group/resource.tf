resource "crusoe_vpc_network" "example" {
  name = "my-vpc-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_subnet" "example" {
  name     = "my-vpc-subnet"
  cidr     = "10.0.0.0/16"
  location = "us-east1-a"
  network  = crusoe_vpc_network.example.id
}

resource "crusoe_instance_template" "example" {
  name     = "my-instance-template"
  type     = "a100-80gb.1x"
  image    = "ubuntu22.04:latest"
  location = "us-east1-a"
  subnet   = crusoe_vpc_subnet.example.id
  ssh_key  = file("~/.ssh/id_ed25519.pub")
}

resource "crusoe_compute_instance_group" "example" {
  name                 = "my-instance-group"
  instance_template_id = crusoe_instance_template.example.id
  desired_count        = 3
}
