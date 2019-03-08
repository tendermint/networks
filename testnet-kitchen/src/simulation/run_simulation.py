#!/usr/local/bin/python

import os
import boto3
import requests

# Simulation parameters
num_seeds = os.environ.get('NUM_SEEDS') if 'NUM_SEEDS' in os.environ else '1'
num_blocks = os.environ.get('NUM_BLOCKS') if 'NUM_BLOCKS' in os.environ else '10'
sim_period = os.environ.get('SIM_PERIOD') if 'SIM_PERIOD' in os.environ else '10'

# Slack details
slack_token = os.environ.get('SLACK_TOKEN')
slack_channel = os.environ.get('SLACK_CHANNEL_ID')
slack_url = 'https://slack.com/api/chat.postMessage'

env = [{'name': 'BLOCKS', 'value': f'{num_blocks}'},
       {'name': 'PERIOD', 'value': f'{sim_period}'},
       {'name': 'SLACK_URL', 'value': slack_url},
       {'name': 'SLACK_TOKEN', 'value': slack_token},
       {'name': 'SLACK_CHANNEL_ID', 'value': slack_channel}]


def push_slack_message(url: str, message: str, thread_ts: str):
    headers = {'Authorization': f'Bearer {slack_token}',
               'Content-Type': 'application/json;charset=UTF-8'}

    raw_respose = requests.post(url, headers=headers, json={'channel': f'{slack_channel}',
                                                            'thread_ts': thread_ts,
                                                            'text': message})
    slack_api_response = raw_respose.json()
    return slack_api_response


# Authenticate with AWS and create ECS client
temp_session = boto3.Session(region_name='us-east-1')
sts_client = temp_session.client('sts')
assume_role_obj = sts_client.assume_role(
    RoleArn='arn:aws:iam::388991194029:role/testnets',
    RoleSessionName='TestnetsRoleSession'
)

aws_credentials = assume_role_obj['Credentials']
session = boto3.Session(region_name='us-east-1',
                        aws_access_key_id=aws_credentials['AccessKeyId'],
                        aws_secret_access_key=aws_credentials['SecretAccessKey'],
                        aws_session_token=aws_credentials['SessionToken'])
ecs = session.client('ecs')

# TODO: something about this hardcoded subnet
network_config = {'awsvpcConfiguration': {'subnets': ['subnet-03a96c201278faf96']}}

slack_reply = push_slack_message(slack_url, "Simulation started", '')
env.append({'name': 'SLACK_MSG_TS', 'value': slack_reply['message']['ts']})

for seed in range(int(num_seeds)):
    env.append({'name': 'SEED', 'value': f'{seed}'})
    result = ecs.run_task(cluster='gaiad-simulation',
                          taskDefinition='simulation:1',
                          launchType='FARGATE',
                          networkConfiguration=network_config,
                          overrides={"containerOverrides": [{"name": "simulation-container",
                                                             "environment": env}]})
    print(result['tasks'][0]['taskArn'])

    if len(result['failures']) != 0 and slack_url != 'None':
        push_slack_message(slack_url, result['failures'][0]['reason'], slack_reply['message']['ts'])
