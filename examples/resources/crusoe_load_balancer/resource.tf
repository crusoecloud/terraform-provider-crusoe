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

# Backends are addressed by the private IP of an existing VM, so the load
# balancer depends on the instances being created first.
resource "crusoe_load_balancer" "example" {
  name     = "my-load-balancer"
  location = "us-east1-a"
  protocol = "tcp"
  vpc_id   = crusoe_vpc_network.example.id

  listen_ports_and_backends = [
    {
      listen_port = 80
      backends = [
        {
          ip   = crusoe_compute_instance.backend_a.network_interfaces[0].private_ipv4.address
          port = 8080
        },
        {
          ip   = crusoe_compute_instance.backend_b.network_interfaces[0].private_ipv4.address
          port = 8080
        }
      ]
    }
  ]
}
