provider "aws" {
  region = "${var.aws_region}"
}

terraform {
  backend "s3" {}
}

data "terraform_remote_state" "network" {
  backend = "s3"

  config {
    bucket = "tendermint-dev-terraform"
    key    = "testnets/network.tfstate"
    region = "${var.aws_region}"
  }
}

resource "aws_ecs_cluster" "testnet" {
  name = "gaiad-simulation"
}

###############################################################
#
# Container configuration templates

data "template_file" "simulation_container" {
  template = "${file("files/container_definitions/simulation.json.tpl")}"

  vars {
    docker_image   = "${var.simulation_image}"
    fargate_cpu    = "${var.cpu_units}"
    fargate_memory = "${var.memory_units}"
    aws_region     = "${var.aws_region}"
    log_group      = "${aws_cloudwatch_log_group.simulation.name}"
    name           = "simulation-container"
  }
}

###############################################################
#
# Task definitions

resource "aws_ecs_task_definition" "stargate" {
  container_definitions    = "${data.template_file.simulation_container.rendered}"
  family                   = "simulation"
  execution_role_arn       = "arn:aws:iam::388991194029:role/ecsTaskExecutionRole"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "${var.cpu_units}"
  memory                   = "${var.memory_units}"
}

###############################################################
#
# Logging configurations

resource "aws_cloudwatch_log_group" "simulation" {
  name              = "simulation"
  retention_in_days = 1

  tags {
    name = "simulation"
  }
}

resource "aws_cloudwatch_log_stream" "stargate" {
  log_group_name = "${aws_cloudwatch_log_group.simulation.name}"
  name           = "simulation-logs"
}
