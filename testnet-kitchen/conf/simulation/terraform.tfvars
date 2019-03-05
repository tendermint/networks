terragrunt = {
  remote_state {
    backend = "s3"

    config {
      bucket         = "tendermint-dev-terraform"
      region         = "us-east-1"
      key            = "gaia-simulation.tfstate"
      encrypt        = true
      dynamodb_table = "gaia-simulation-state-lock"
    }
  }

  terraform {
    source = "../../src//simulation"
  }
}
