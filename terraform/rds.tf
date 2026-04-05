resource "aws_db_subnet_group" "main" {
  name       = "realworld-db-subnet-group"
  subnet_ids = [aws_subnet.private-a.id, aws_subnet.private-b.id]

  tags = { Name = "realworld-db-subnet-group" }
}

resource "aws_db_instance" "main" {
  identifier        = "realworld-db"
  engine            = "postgres"
  engine_version    = "17"
  instance_class    = "db.t3.micro"
  allocated_storage = 20

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  skip_final_snapshot = true
  publicly_accessible = false

  tags = { Name = "realworld-db" }
}
