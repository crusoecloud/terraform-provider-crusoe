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

resource "crusoe_storage_disk" "example" {
  name     = "my-data-disk"
  size     = "100GiB"
  location = "us-east1-a"
}

resource "crusoe_compute_instance" "example" {
  name     = "my-vm"
  type     = "a100-80gb.1x"
  image    = "ubuntu22.04:latest"
  location = "us-east1-a"

  disks = [
    {
      id              = crusoe_storage_disk.example.id
      mode            = "read-only"
      attachment_type = "data"
    }
  ]

  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.example.id
    }
  ]

  ssh_key = file("~/.ssh/id_ed25519.pub")
}
