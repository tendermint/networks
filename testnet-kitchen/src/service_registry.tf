/*resource "aws_service_discovery_private_dns_namespace" "testnet" {
  count = "${length(var.chain_ids)}"
  name  = "${element(var.chain_ids, count.index)}"
  vpc   = "${aws_vpc.testnet.id}"
}

resource "aws_service_discovery_service" "validator" {
  count = "${length(var.chain_ids)}"

  dns_config {
    dns_records {
      ttl  = 10
      type = "SRV"
    }

    routing_policy = "MULTIVALUE"

    namespace_id = "${element(aws_service_discovery_private_dns_namespace.testnet.*.id, count.index)}"
  }

  health_check_custom_config {
    failure_threshold = 1
  }

  name = "${element(var.chain_ids, count.index)}"
}
*/

