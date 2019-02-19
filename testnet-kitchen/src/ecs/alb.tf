resource "aws_lb" "testnet" {
  name               = "${var.chain_id}"
  load_balancer_type = "application"
  internal           = false

  subnets = [
    "${data.terraform_remote_state.network.priv_subnet_id}",
    "${data.terraform_remote_state.network.pub_subnet_id}",
  ]

  security_groups = ["${data.terraform_remote_state.network.alb_sec_grp_id}"]
}

resource "aws_lb_target_group" "testnet_validators" {
  name        = "validators"
  target_type = "instance"
  protocol    = "HTTP"
  vpc_id      = "${data.terraform_remote_state.network.vpc_id}"
  port        = "${var.rpc_port}"
  depends_on  = ["aws_lb.testnet"]
}

resource "aws_lb_target_group" "testnet_stargate" {
  name        = "stargate"
  target_type = "instance"
  protocol    = "HTTP"
  vpc_id      = "${data.terraform_remote_state.network.vpc_id}"
  port        = "${var.stargate_port}"
  depends_on  = ["aws_lb.testnet"]

  health_check {
    path    = "/node_info"
    matcher = "200"
  }
}

resource "aws_lb_listener" "testnet_stargate" {
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.testnet_stargate.arn}"
  }

  load_balancer_arn = "${aws_lb.testnet.arn}"
  port              = "${var.stargate_port}"
  protocol          = "HTTP"
}

resource "aws_lb_listener" "testnet_validators" {
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.testnet_validators.arn}"
  }

  load_balancer_arn = "${aws_lb.testnet.arn}"
  port              = "${var.rpc_port}"
  protocol          = "HTTP"
}
