provider "aws" {
  region = "${var.aws_region}"
}

terraform {
  backend "s3" {}
}

data "aws_availability_zones" available {}

resource "aws_vpc" "testnet" {
  cidr_block = "10.20.0.0/16"

  enable_dns_hostnames = true
  enable_dns_support   = true

  tags {
    Name = "testnets-ecs-ec2"
  }
}

resource "aws_internet_gateway" "gateway" {
  vpc_id = "${aws_vpc.testnet.id}"

  tags = {
    Name = "testnets-ecs-ec2"
  }
}

resource "aws_eip" "nat" {
  vpc = true

  tags {
    Name = "testnets-ecs-ec2"
  }
}

resource "aws_nat_gateway" "testnet_container_instance" {
  allocation_id = "${aws_eip.nat.id}"
  subnet_id     = "${aws_subnet.testnet_public.id}"
}

resource "aws_subnet" "testnet_public" {
  cidr_block              = "${cidrsubnet(aws_vpc.testnet.cidr_block, 8, 1)}"
  availability_zone_id    = "${data.aws_availability_zones.available.zone_ids[0]}"
  vpc_id                  = "${aws_vpc.testnet.id}"
  map_public_ip_on_launch = true

  tags {
    Name = "testnets-ecs-ec2-public"
  }
}

resource "aws_subnet" "testnet_private" {
  cidr_block              = "${cidrsubnet(aws_vpc.testnet.cidr_block, 8, 2)}"
  vpc_id                  = "${aws_vpc.testnet.id}"
  map_public_ip_on_launch = false

  # TODO: better/any AZ distribution
  availability_zone_id = "${data.aws_availability_zones.available.zone_ids[1]}"

  tags {
    Name = "testnets-ecs-ec2-private"
  }
}

resource "aws_route_table" "testnet_public_subnet" {
  vpc_id = "${aws_vpc.testnet.id}"

  tags {
    Name = "testnets-ecs-ec2-subnet"
  }
}

resource "aws_route_table" "testnet_private_subnet" {
  vpc_id = "${aws_vpc.testnet.id}"

  tags {
    Name = "testnets-ecs-ec2-private-subnet"
  }
}

resource "aws_route" "testnet_internet_gateway" {
  route_table_id         = "${aws_route_table.testnet_public_subnet.id}"
  gateway_id             = "${aws_internet_gateway.gateway.id}"
  destination_cidr_block = "0.0.0.0/0"
}

resource "aws_route" "testnet_nat" {
  route_table_id         = "${aws_route_table.testnet_private_subnet.id}"
  nat_gateway_id         = "${aws_nat_gateway.testnet_container_instance.id}"
  destination_cidr_block = "0.0.0.0/0"
}

resource "aws_route_table_association" "testnet_public_subnet" {
  subnet_id      = "${aws_subnet.testnet_public.id}"
  route_table_id = "${aws_route_table.testnet_public_subnet.id}"
}

resource "aws_route_table_association" "testnet_nat" {
  route_table_id = "${aws_route_table.testnet_private_subnet.id}"
  subnet_id      = "${aws_subnet.testnet_private.id}"
}
