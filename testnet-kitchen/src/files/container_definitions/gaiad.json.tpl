[{
 	"name": "${name}",
 	"image": "${docker_image}",
 	"cpu": ${fargate_cpu},
 	"memory": ${fargate_memory},
 	"networkMode": "awsvpc",
 	"logConfiguration": {
 		"logDriver": "awslogs",
 		"options": {
 			"awslogs-group": "${log_group}",
 			"awslogs-region": "${aws_region}",
 			"awslogs-stream-prefix": "ecs"
 		}
 	},
 	"portMappings": [{
 		"containerPort": ${rpc_port},
 		"hostPort": 0
 	}],
 	"mountPoints": [
 	    {
 	        "containerPath": "/config",
 	        "sourceVolume": "efs-config"
 	    }
 	],
 	"environment": [
 	    { "name" : "MONIKER", "value" : "${moniker}" },
 	    { "name" : "PERSISTENT_PEERS", "value" : "${persistent_peers}" }
 	],
 	"command": ["start"]
 }]