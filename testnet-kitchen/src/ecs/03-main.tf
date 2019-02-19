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
  name = "${var.chain_id}"
}

###############################################################
#
# Container configuration templates

data "template_file" "reference_validator" {
  template = "${file("files/container_definitions/gaiad.json.tpl")}"

  vars {
    moniker          = "${var.moniker}-validator"
    docker_image     = "${var.gaiad_image}"
    fargate_memory   = "${var.gaiad_memory}"
    fargate_cpu      = "${var.gaiad_cpu}"
    aws_region       = "${var.aws_region}"
    rpc_port         = "${var.rpc_port}"
    persistent_peers = "${aws_lb.testnet.dns_name}:${var.rpc_port}"
    log_group        = "${aws_cloudwatch_log_group.reference_validator.name}"
    name             = "${var.chain_id}-reference-validator"
  }
}

data "template_file" "stargate_container" {
  template = "${file("files/container_definitions/stargate.json.tpl")}"

  vars {
    docker_image   = "${var.stargate_image}"
    fargate_cpu    = "${var.stargate_cpu}"
    fargate_memory = "${var.stargate_memory}"
    aws_region     = "${var.aws_region}"
    gaiad_service  = "${aws_lb.testnet.dns_name}"
    stargate_port  = "${var.stargate_port}"
    log_group      = "${aws_cloudwatch_log_group.stargate.name}"
    name           = "${var.chain_id}-stargate"
  }
}

###############################################################
#
# Task definitions

resource "aws_ecs_task_definition" "reference_validator" {
  container_definitions    = "${data.template_file.reference_validator.rendered}"
  family                   = "${var.chain_id}-reference-validator"
  execution_role_arn       = "${var.ecs_task_execution_role}"
  requires_compatibilities = ["EC2"]
  cpu                      = "${var.gaiad_cpu}"
  memory                   = "${var.gaiad_memory}"

  volume {
    name      = "efs-config"
    host_path = "/config"
  }
}

resource "aws_ecs_task_definition" "stargate" {
  container_definitions    = "${data.template_file.stargate_container.rendered}"
  family                   = "${var.chain_id}-stargate"
  execution_role_arn       = "${var.ecs_task_execution_role}"
  requires_compatibilities = ["EC2"]
  cpu                      = "${var.stargate_cpu}"
  memory                   = "${var.stargate_memory}"
}

###############################################################
#
# Service configurations

resource "aws_ecs_service" "reference_validator" {
  cluster         = "${aws_ecs_cluster.testnet.id}"
  name            = "${var.chain_id}-reference-validator"
  task_definition = "${aws_ecs_task_definition.reference_validator.arn}"
  launch_type     = "EC2"
  desired_count   = "${var.num_val_nodes}"

  load_balancer {
    target_group_arn = "${aws_lb_target_group.testnet_validators.arn}"
    container_name   = "${data.template_file.reference_validator.vars["name"]}"
    container_port   = "${var.rpc_port}"
  }
}

resource "aws_ecs_service" "stargate" {
  cluster         = "${aws_ecs_cluster.testnet.id}"
  name            = "${var.chain_id}-stargate"
  task_definition = "${aws_ecs_task_definition.stargate.arn}"
  launch_type     = "EC2"
  desired_count   = "${var.stargate_instances}"

  load_balancer {
    target_group_arn = "${aws_lb_target_group.testnet_stargate.arn}"
    container_name   = "${data.template_file.stargate_container.vars["name"]}"
    container_port   = "${var.stargate_port}"
  }
}

###############################################################
#
# Logging configurations

resource "aws_cloudwatch_log_group" "reference_validator" {
  name              = "${var.chain_id}-reference-validator"
  retention_in_days = 3

  tags {
    name = "${var.chain_id}-reference-validator"
  }
}

resource "aws_cloudwatch_log_stream" "reference_validator" {
  log_group_name = "${aws_cloudwatch_log_group.reference_validator.name}"
  name           = "${var.chain_id}-reference-validator"
}

resource "aws_cloudwatch_log_group" "stargate" {
  name              = "${var.chain_id}-stargate"
  retention_in_days = 3

  tags {
    name = "${var.chain_id}-stargate"
  }
}

resource "aws_cloudwatch_log_stream" "stargate" {
  log_group_name = "${aws_cloudwatch_log_group.stargate.name}"
  name           = "${var.chain_id}-stargate"
}
