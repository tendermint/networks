#!/usr/local/bin/python

import boto3


def assume_role():
    temp_session = boto3.Session(region_name='us-east-1')
    sts_client = temp_session.client('sts')
    assume_role_obj = sts_client.assume_role(
        RoleArn='arn:aws:iam::388991194029:role/testnets',
        RoleSessionName='AssumeRoleSession1'
    )

    return assume_role_obj['Credentials']


credentials = assume_role()
session = boto3.Session(region_name='us-east-1',
                        aws_access_key_id=credentials['AccessKeyId'],
                        aws_secret_access_key=credentials['SecretAccessKey'],
                        aws_session_token=credentials['SessionToken'])

ecs = session.client('ecs')

network_config = {'awsvpcConfiguration': {'subnets': ['subnet-03a96c201278faf96']}}

ecs.run_task(cluster='gaiad-simulation',
             taskDefinition='simulation:1',
             launchType='FARGATE',
             networkConfiguration=network_config)
