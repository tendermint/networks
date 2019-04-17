data "aws_ami" "ecs_ami" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["*amazon-ecs-optimized"]
  }

  filter {
    name   = "owner-alias"
    values = ["amazon"]
  }
}

data "template_cloudinit_config" "container_instance" {
  gzip          = true
  base64_encode = true

  part {
    content_type = "text/x-shellscript"

    content = <<EOF
#!/bin/bash
echo ECS_CLUSTER=${aws_ecs_cluster.testnet.name} >> /etc/ecs/ecs.config
EOF
  }
}

data "aws_availability_zones" "available" {}

resource "aws_autoscaling_group" "simulation" {
  name                 = "long-simulation"
  launch_configuration = "${aws_launch_configuration.container_instance.name}"
  availability_zones   = ["${data.aws_availability_zones.available.names}"]
  max_size             = 300
  min_size             = 0

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_launch_configuration" "container_instance" {
  name_prefix   = "simulation-lc-"
  image_id      = "${data.aws_ami.ecs_ami.id}"
  instance_type = "t2.small"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_iam_role" "ecs_instance_role" {
  name               = "SimulationEcsInstanceRole"
  assume_role_policy = "${data.aws_iam_policy_document.container_instance_role_trust.json}"
  path               = "/"
}

resource "aws_iam_role_policy" "ecs_instance" {
  policy = "${data.aws_iam_policy_document.container_instance.json}"
  role   = "${aws_iam_role.ecs_instance_role.id}"
}

data "aws_iam_policy_document" "container_instance" {
  statement {
    sid = "AmazonEC2ContainerServiceforEC2Role"

    effect = "Allow"

    actions = [
      "logs:PutLogEvents",
      "logs:CreateLogStream",
      "ecs:Submit*",
      "ecs:StartTelemetrySession",
      "ecs:RegisterContainerInstance",
      "ecs:Poll",
      "ecs:DiscoverPollEndpoint",
      "ecs:DeregisterContainerInstance",
      "ecs:CreateCluster",
      "ecr:GetDownloadUrlForLayer",
      "ecr:GetAuthorizationToken",
      "ecr:BatchGetImage",
      "ecr:BatchCheckLayerAvailability",
    ]

    resources = ["*"]
  }
}

data "aws_iam_policy_document" "container_instance_role_trust" {
  statement {
    effect = "Allow"

    principals {
      identifiers = ["ec2.amazonaws.com"]
      type        = "Service"
    }

    actions = ["sts:AssumeRole"]
  }
}
