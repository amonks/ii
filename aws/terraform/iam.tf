resource "aws_iam_user" "monks-go" {
  name = "monks-go_iam_user"
}



resource "aws_iam_access_key" "monks-go" {
  user = aws_iam_user.monks-go.name
}


resource "aws_iam_policy" "send_ss_cx_emails" {
  name = "send_ss_cx_emails"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Action": [
        "ses:SendRawEmail"
      ],
      "Resource": [
        "arn:aws:ses:us-east-1:558796306206:identity/no-reply@mail.ss.cx"
      ]
    }
  ]
}
EOF
}

resource "aws_iam_policy" "write_dns_records" {
  name = "write_dns_records_for_acme_challenge"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Action": [
        "route53:ListResourceRecordSets",
        "route53:GetChange",
        "route53:ChangeResourceRecordSets"
      ],
      "Resource": [
        "arn:aws:route53:::hostedzone/*",
        "arn:aws:route53:::change/*"
      ]
    },
    {
      "Sid": "",
      "Effect": "Allow",
      "Action": [
        "route53:ListHostedZonesByName",
        "route53:ListHostedZones"
      ],
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_user_policy_attachment" "monks-go_iam_user_write_dns_records" {
  user       = aws_iam_user.monks-go.name
  policy_arn = aws_iam_policy.write_dns_records.arn
}

