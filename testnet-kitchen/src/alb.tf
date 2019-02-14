resource "aws_lb" "testnet" {
  count              = "${length(var.chain_ids)}"
  name               = "${element(var.chain_ids, count.index)}"
  load_balancer_type = "application"
  internal           = false

  subnets = [
    "${element(aws_subnet.testnet_public.*.id, count.index)}",
    "${element(aws_subnet.testnet_private.*.id, count.index)}",
  ]

  security_groups = ["${aws_security_group.alb.id}"]
}

resource "aws_lb_target_group" "testnet_validators" {
  name        = "validators"
  target_type = "instance"
  protocol    = "HTTP"
  vpc_id      = "${aws_vpc.testnet.id}"
  port        = "26657"
  depends_on  = ["aws_lb.testnet"]
}

resource "aws_lb_target_group" "testnet_stargate" {
  name        = "stargate"
  target_type = "instance"
  protocol    = "HTTP"
  vpc_id      = "${aws_vpc.testnet.id}"
  port        = "1317"
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
  port              = "1317"
  protocol          = "HTTP"
}

resource "aws_lb_listener" "testnet_validators" {
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.testnet_validators.arn}"
  }

  load_balancer_arn = "${aws_lb.testnet.arn}"
  port              = "26657"
  protocol          = "HTTP"
}
