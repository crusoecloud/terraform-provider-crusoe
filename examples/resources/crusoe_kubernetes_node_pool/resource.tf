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

resource "crusoe_kubernetes_cluster" "example" {
  name      = "my-cluster"
  version   = "1.31.7-cmk.7"
  location  = "us-east1-a"
  subnet_id = crusoe_vpc_subnet.example.id
}

resource "crusoe_kubernetes_node_pool" "example" {
  name           = "my-node-pool"
  cluster_id     = crusoe_kubernetes_cluster.example.id
  instance_count = 2
  type           = "a100-80gb.1x"
  ssh_key        = file("~/.ssh/id_ed25519.pub")
}
