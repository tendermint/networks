# CircleCI  config  
Trigger this job through the API with the following command
```bash
curl --silent \\
     --show-error \\
     -X POST \\
     --header "Content-Type: application/json" -d "{\"build_parameters\": {\"CONFIG_TOML\": \"$BASE_64_ENCODED_FILE\"}}" "https://circleci.com/api/v1.1/project/github/tendermint/networks/tree/$GIT_BRANCH?circle-token=$CIRCLECI_API_TOKEN"
```
Ensure that `BASE_64_ENCODED_FILE`, `GIT_BRANCH` and `CIRCLECI_API_TOKEN` are replaced with appropriate values

Manage / obtain a [CircleCI token](https://circleci.com/docs/2.0/managing-api-tokens/)