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
    Name = "EIP for NAT"
  }
}

resource "aws_nat_gateway" "testnet_container_instance" {
  allocation_id = "${aws_eip.nat.id}"
  subnet_id     = "${aws_subnet.testnet_public.id}"
}

resource "aws_subnet" "testnet_public" {
  count                   = "${length(var.chain_ids)}"
  cidr_block              = "${cidrsubnet(aws_vpc.testnet.cidr_block, 8, length(var.chain_ids) + count.index)}"
  availability_zone_id    = "${data.aws_availability_zones.available.zone_ids[0]}"
  vpc_id                  = "${aws_vpc.testnet.id}"
  map_public_ip_on_launch = true

  tags {
    Name = "${element(var.chain_ids, count.index)}-public"
  }
}

resource "aws_subnet" "testnet_private" {
  count                   = "${length(var.chain_ids)}"
  cidr_block              = "${cidrsubnet(aws_vpc.testnet.cidr_block, 8, length(var.chain_ids) * 2 + count.index)}"
  vpc_id                  = "${aws_vpc.testnet.id}"
  map_public_ip_on_launch = false

  # TODO: better/any AZ distribution
  availability_zone_id = "${data.aws_availability_zones.available.zone_ids[1]}"

  tags {
    Name = "${element(var.chain_ids, count.index)}-private"
  }
}

resource "aws_route_table" "testnet_public_subnet" {
  vpc_id = "${aws_vpc.testnet.id}"

  tags {
    Name = "testnets-public-subnet"
  }
}

resource "aws_route_table" "testnet_private_subnet" {
  vpc_id = "${aws_vpc.testnet.id}"

  tags {
    Name = "testnets-private-subnet"
  }
}

resource "aws_route" "testnet_internet_gateway" {
  count                  = "${length(var.chain_ids)}"
  route_table_id         = "${aws_route_table.testnet_public_subnet.id}"
  gateway_id             = "${aws_internet_gateway.gateway.id}"
  destination_cidr_block = "0.0.0.0/0"
}

resource "aws_route" "testnet_nat" {
  count                  = "${length(var.chain_ids)}"
  route_table_id         = "${aws_route_table.testnet_private_subnet.id}"
  nat_gateway_id         = "${aws_nat_gateway.testnet_container_instance.id}"
  destination_cidr_block = "0.0.0.0/0"
}

resource "aws_route_table_association" "testnet_public_subnet" {
  count          = "${length(var.chain_ids)}"
  subnet_id      = "${element(aws_subnet.testnet_public.*.id, count.index)}"
  route_table_id = "${aws_route_table.testnet_public_subnet.id}"
}

resource "aws_route_table_association" "testnet_nat" {
  count          = "${length(var.chain_ids)}"
  route_table_id = "${aws_route_table.testnet_private_subnet.id}"
  subnet_id      = "${element(aws_subnet.testnet_private.*.id, count.index)}"
}
