resource "aws_security_group" "alb" {
  name   = "testnets-alb"
  vpc_id = "${aws_vpc.testnet.id}"

  ingress {
    protocol    = "tcp"
    from_port   = "26657"
    to_port     = "26657"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 22
    protocol    = "tcp"
    to_port     = 22
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    protocol    = "tcp"
    from_port   = "1317"
    to_port     = "1317"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol    = "-1"
    from_port   = "0"
    to_port     = "0"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "instance" {
  name   = "testnets-instance"
  vpc_id = "${aws_vpc.testnet.id}"

  ingress {
    protocol        = "tcp"
    from_port       = "32768"
    to_port         = "65535"
    security_groups = ["${aws_security_group.alb.id}"]
  }

  ingress {
    from_port       = "22"
    protocol        = "tcp"
    to_port         = "22"
    security_groups = ["${aws_security_group.alb.id}"]
  }

  egress {
    protocol    = "-1"
    from_port   = "0"
    to_port     = "0"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "efs_access" {
  description = "EFS access from container instance"
  vpc_id      = "${aws_vpc.testnet.id}"

  ingress {
    protocol        = "-1"
    from_port       = "0"
    to_port         = "0"
    security_groups = ["${aws_security_group.instance.id}"]
  }
}
