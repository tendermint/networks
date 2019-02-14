provider "aws" {
  region = "${var.aws_region}"
}

terraform {
  backend "s3" {}
}

resource "aws_ecs_cluster" "testnet" {
  count = "${length(var.chain_ids)}"
  name  = "${element(var.chain_ids, count.index)}"
}

###############################################################
#
# Container configuration templates

data "template_file" "reference_validator" {
  count    = "${length(var.chain_ids)}"
  template = "${file("files/container_definitions/gaiad.json.tpl")}"

  vars {
    docker_image     = "${var.gaiad_image}"
    fargate_cpu      = "${var.gaiad_cpu}"
    fargate_memory   = "${var.gaiad_memory}"
    aws_region       = "${var.aws_region}"
    log_group        = "${element(aws_cloudwatch_log_group.reference_validator.*.name, count.index)}"
    name             = "${element(var.chain_ids, count.index)}-reference-validator"
    rpc_port         = "${var.rpc_port}"
    moniker          = "${var.moniker}-validator-${count.index}"
    persistent_peers = "${aws_lb.testnet.dns_name}:${var.rpc_port}"
  }
}

data "template_file" "stargate_container" {
  count    = "${length(var.chain_ids)}"
  template = "${file("files/container_definitions/stargate.json.tpl")}"

  vars {
    docker_image   = "${var.stargate_image}"
    fargate_cpu    = "${var.stargate_cpu}"
    fargate_memory = "${var.stargate_memory}"
    aws_region     = "${var.aws_region}"
    log_group      = "${element(aws_cloudwatch_log_group.stargate.*.name, count.index)}"
    name           = "${element(var.chain_ids, count.index)}-stargate"
    gaiad_service  = "${element(aws_lb.testnet.*.dns_name, count.index)}"
    stargate_port  = "1317"
  }
}

###############################################################
#
# Task definitions

resource "aws_ecs_task_definition" "reference_validator" {
  count                    = "${length(var.chain_ids)}"
  container_definitions    = "${element(data.template_file.reference_validator.*.rendered, count.index)}"
  family                   = "${element(var.chain_ids, count.index)}-reference-validator"
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
  count                    = "${length(var.chain_ids)}"
  container_definitions    = "${element(data.template_file.stargate_container.*.rendered, count.index)}"
  family                   = "${element(var.chain_ids, count.index)}-stargate"
  execution_role_arn       = "${var.ecs_task_execution_role}"
  requires_compatibilities = ["EC2"]
  cpu                      = "${var.stargate_cpu}"
  memory                   = "${var.stargate_memory}"
}

###############################################################
#
# Service configurations

resource "aws_ecs_service" "reference_validator" {
  count           = "${length(var.chain_ids)}"
  cluster         = "${element(aws_ecs_cluster.testnet.*.id, count.index)}"
  name            = "${element(var.chain_ids, count.index)}-reference-validator"
  task_definition = "${element(aws_ecs_task_definition.reference_validator.*.arn, count.index)}"
  launch_type     = "EC2"
  desired_count   = "${var.num_val_nodes}"

  load_balancer {
    target_group_arn = "${aws_lb_target_group.testnet_validators.arn}"
    container_name   = "${data.template_file.reference_validator.vars.name}"
    container_port   = "${var.rpc_port}"
  }
}

resource "aws_ecs_service" "stargate" {
  count           = "${length(var.chain_ids)}"
  cluster         = "${element(aws_ecs_cluster.testnet.*.id, count.index)}"
  name            = "${element(var.chain_ids, count.index)}-stargate"
  task_definition = "${element(aws_ecs_task_definition.stargate.*.arn, count.index)}"
  launch_type     = "EC2"
  desired_count   = "${var.stargate_instances}"

  load_balancer {
    target_group_arn = "${aws_lb_target_group.testnet_stargate.arn}"
    container_name   = "${data.template_file.stargate_container.vars.name}"
    container_port   = "1317"
  }
}

###############################################################
#
# Logging configurations

resource "aws_cloudwatch_log_group" "reference_validator" {
  count             = "${length(var.chain_ids)}"
  name              = "${element(var.chain_ids, count.index)}-reference-validator"
  retention_in_days = 3

  tags {
    name = "${element(var.chain_ids, count.index)}-reference-validator"
  }
}

resource "aws_cloudwatch_log_stream" "reference_validator" {
  count          = "${length(var.chain_ids)}"
  log_group_name = "${element(aws_cloudwatch_log_group.reference_validator.*.name, count.index)}"
  name           = "${element(var.chain_ids, count.index)}-reference-validator"
}

resource "aws_cloudwatch_log_group" "stargate" {
  count             = "${length(var.chain_ids)}"
  name              = "${element(var.chain_ids, count.index)}-stargate"
  retention_in_days = 3

  tags {
    name = "${element(var.chain_ids, count.index)}-stargate"
  }
}

resource "aws_cloudwatch_log_stream" "stargate" {
  count          = "${length(var.chain_ids)}"
  log_group_name = "${element(aws_cloudwatch_log_group.stargate.*.name, count.index)}"
  name           = "${element(var.chain_ids, count.index)}-stargate"
}
