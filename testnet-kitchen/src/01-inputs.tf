####################################################################################

variable "chain_id" {
  type    = "string"
  default = "testnet"
}

variable "chain_ids" {
  type = "list"
}

variable "aws_region" {
  type    = "string"
  default = "us-east-1"
}

variable "num_val_nodes" {
  type        = "string"
  description = "The number of validator nodes to create"
  default     = "2"
}

variable "num_seed_nodes" {
  type        = "string"
  description = "The number of seed nodes to create"
  default     = "0"
}

variable "num_full_nodes" {
  type        = "string"
  description = "The number of seed nodes to create"
  default     = "0"
}

####################################################################################
####################################################################################
variable "ecs_task_execution_role" {
  description = "Role arn for the ecsTaskExecutionRole"
  default     = "arn:aws:iam::388991194029:role/ecsTaskExecutionRole"
}

variable "gaiad_image" {
  description = "Gaiad docker image"
  default     = "testnets/gaiad_validator_ecs"
}

variable "gaiad_cpu" {
  description = "Fargate instance CPU units to provision (1 vCPU = 1024 CPU units)"
  default     = "1024"
}

variable "gaiad_memory" {
  description = "Fargate instance memory to provision (in MiB)"
  default     = "1024"
}

variable "rpc_port" {
  description = "gaiad RPC port"
  default     = "26657"
}

variable "stargate_instances" {
  description = "Number of nodes stargate instances"
  default     = "1"
}

variable "stargate_image" {
  description = "Stargate docker image"
  default     = "tendermint/ecs_gaiacli"
}

variable "stargate_cpu" {
  description = "Fargate instance CPU units to provision (1 vCPU = 1024 CPU units)"
  default     = "256"
}

variable "stargate_memory" {
  description = "Fargate instance memory to provision (in MiB)"
  default     = "512"
}

variable "moniker" {
  type    = "string"
  default = "testnet-kitchen"
}
