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
