resource "aws_eip" "nat-a" {
  domain = "vpc"

  tags = {
    Name = "nat-eip-a"
  }
}

resource "aws_eip" "nat-b" {
  domain = "vpc"

  tags = {
    Name = "nat-eip-b"
  }
}

resource "aws_nat_gateway" "nat-a" {
  allocation_id = aws_eip.nat-a.id
  subnet_id     = aws_subnet.public-a.id

  tags = {
    Name = "nat-gw-a"
  }

  depends_on = [aws_internet_gateway.gw]
}

resource "aws_nat_gateway" "nat-b" {
  allocation_id = aws_eip.nat-b.id
  subnet_id     = aws_subnet.public-b.id

  tags = {
    Name = "nat-gw-b"
  }

  depends_on = [aws_internet_gateway.gw]
}

resource "aws_route_table" "private-rt-a" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.nat-a.id
  }

  tags = {
    Name = "private-route-table-a"
  }
}

resource "aws_route_table" "private-rt-b" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.nat-b.id
  }

  tags = {
    Name = "private-route-table-b"
  }
}

resource "aws_route_table_association" "private-a" {
  subnet_id      = aws_subnet.private-a.id
  route_table_id = aws_route_table.private-rt-a.id
}

resource "aws_route_table_association" "private-b" {
  subnet_id      = aws_subnet.private-b.id
  route_table_id = aws_route_table.private-rt-b.id
}
