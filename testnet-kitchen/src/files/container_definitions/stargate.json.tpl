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
 			"awslogs-stream-prefix": "stargate"
 		}
 	},
 	"portMappings": [{
 		"containerPort": ${stargate_port},
 		"hostPort": 0
 	}],
 	"command": [
 	    "rest-server",
 	    "--insecure",
 	    "--trust-node=true",
 	    "--node=${gaiad_service}:26657",
 	    "--laddr=tcp://0.0.0.0:${stargate_port}"
 	]
 }]