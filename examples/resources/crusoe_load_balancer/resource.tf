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

resource "crusoe_compute_instance" "backend_a" {
  name     = "my-backend-a"
  type     = "a100-80gb.1x"
  image    = "ubuntu22.04:latest"
  location = "us-east1-a"
  ssh_key  = file("~/.ssh/id_ed25519.pub")

  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.example.id
    }
  ]
}

resource "crusoe_compute_instance" "backend_b" {
  name     = "my-backend-b"
  type     = "a100-80gb.1x"
  image    = "ubuntu22.04:latest"
  location = "us-east1-a"
  ssh_key  = file("~/.ssh/id_ed25519.pub")

  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.example.id
    }
  ]
}

resource "crusoe_load_balancer" "example" {
  name      = "my-load-balancer"
  algorithm = "random"
  location  = "us-east1-a"
  protocols = ["tcp"]

  destinations = [
    {
      resource_id = crusoe_compute_instance.backend_a.id
    },
    {
      resource_id = crusoe_compute_instance.backend_b.id
    }
  ]

  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.example.id
    }
  ]
}
