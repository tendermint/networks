variable "aws_region" {
  type    = "string"
  default = "us-east-1"
}

variable "simulation_image" {
  description = "Docker image that will be used by the task"
  type        = "string"
  default     = "tendermintdev/gaia_sim"
}

variable "cpu_units" {
  description = "Fargate instance CPU units to provision (1 vCPU = 1024 CPU units)"
  type        = "string"
  default     = "1024"
}

variable "memory_units" {
  description = "Fargate instance memory units to provision (in MiB)"
  type        = "string"
  default     = "2048"
}
