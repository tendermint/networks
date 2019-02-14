terragrunt = {
  iam_role = "arn:aws:iam::388991194029:role/testnets"

  remote_state {
    backend = "s3"

    config {
      bucket         = "tendermint-dev-terraform"
      region         = "us-east-1"
      key            = "testnets/ecs_ec2.tfstate"
      encrypt        = true
      dynamodb_table = "testnets-state-lock"
    }
  }

  terraform {
    source = "..//src"
  }
}

chain_ids = ["voyager-2"]

gaiad_memory = "4046"