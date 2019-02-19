output "test_config_template" {
  value = "${data.template_cloudinit_config.container_instance.rendered}"
}
