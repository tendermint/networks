data "aws_ami" "ecs_ami" {
  most_recent = true
  owners = ["amazon"]

  filter {
    name   = "name"
    values = ["*amazon-ecs-optimized"]
  }

  filter {
    name   = "owner-alias"
    values = ["amazon"]
  }
}

resource "aws_instance" "container_instance" {
  instance_type          = "t2.xlarge"
  ami                    = "${data.aws_ami.ecs_ami.id}"
  key_name               = "gos-seed"
  subnet_id              = "${data.terraform_remote_state.network.priv_subnet_id}"
  iam_instance_profile   = "${aws_iam_instance_profile.testnet.id}"
  vpc_security_group_ids = ["${data.terraform_remote_state.network.container_instance_sec_grp_id}"]
  user_data              = "${data.template_cloudinit_config.container_instance.rendered}"

  tags {
    Name = "Testnet Container Instance"
  }
}

###############################################################
#
# Cloud init

data "template_cloudinit_config" "container_instance" {
  gzip          = true
  base64_encode = true

  part {
    content_type = "text/cloud-boothook"

    content = <<EOF
# Install nfs-utils
cloud-init-per once yum_update yum update -y
cloud-init-per once install_nfs_utils yum install -y nfs-utils

# Create /config folder
cloud-init-per once mkdir_efs mkdir /config

# Mount /config
cloud-init-per once mount_efs echo -e '${aws_efs_file_system.config_files.id}.efs.${var.aws_region}.amazonaws.com:/ /config nfs4 nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 0 0' >> /etc/fstab
mount -a
EOF
  }

  part {
    content_type = "text/cloud-config"

    content = <<EOF
write_files:
    - path: /config/config.toml
      permissions: '0644'
      encoding: base64

      # file is provided by circleci already base64 encoded
      content: ${file("files/config.toml")}
EOF
  }

  part {
    content_type = "text/x-shellscript"

    content = <<EOF
#! /bin/bash
echo ECS_CLUSTER=${aws_ecs_cluster.testnet.name} >> /etc/ecs/ecs.config
EOF
  }
}

###############################################################
#
# Instance profile

resource "aws_iam_instance_profile" "testnet" {
  name = "testnet-profile"
  role = "${aws_iam_role.ecs_instance_role.name}"
}

###############################################################
#
# Instance role

resource "aws_iam_role" "ecs_instance_role" {
  name               = "ecsInstanceRole"
  assume_role_policy = "${data.aws_iam_policy_document.container_instance_role_trust.json}"
  path               = "/"
}

resource "aws_iam_role_policy" "ecs_instance" {
  name   = "AmazonEC2ContainerServiceforEC2Role"
  policy = "${data.aws_iam_policy_document.container_instance.json}"
  role   = "${aws_iam_role.ecs_instance_role.id}"
}

###############################################################
#
# Elastic file system

resource "aws_efs_file_system" "config_files" {
  creation_token = "config_file_system"

  tags {
    Name = "testnets-config"
  }
}

resource "aws_efs_mount_target" "config_files" {
  file_system_id  = "${aws_efs_file_system.config_files.id}"
  subnet_id       = "${data.terraform_remote_state.network.priv_subnet_id}"
  security_groups = ["${data.terraform_remote_state.network.efs_sec_grp_id}"]
}

###############################################################
#
# Policy documents

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
