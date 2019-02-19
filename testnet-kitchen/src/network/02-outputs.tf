output "vpc_id" {
  value = "${aws_vpc.testnet.id}"
}

output "efs_sec_grp_id" {
  value = "${aws_security_group.efs_access.id}"
}

output "alb_sec_grp_id" {
  value = "${aws_security_group.alb.id}"
}

output "container_instance_sec_grp_id" {
  value = "${aws_security_group.instance.id}"
}

output "priv_subnet_id" {
  value = "${aws_subnet.testnet_private.id}"
}

output "pub_subnet_id" {
  value = "${aws_subnet.testnet_public.id}"
}
