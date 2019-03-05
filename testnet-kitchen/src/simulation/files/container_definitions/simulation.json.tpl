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
 	}
 }]