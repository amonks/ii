output "monks-go_iam_user_access_key_id" {
  value = aws_iam_access_key.monks-go.id
}

output "monks-go_iam_user_secret_access_key" {
  value = aws_iam_access_key.monks-go.secret
}



resource "aws_iam_user" "monks-go" {
  name = "monks-go_iam_user"
}



resource "aws_iam_access_key" "monks-go" {
  user = aws_iam_user.monks-go.name
}



resource "aws_iam_policy" "write_ss_cx_records" {
  name = "write_dns_records_for_ss_cx_acme_challenge"

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
        "arn:aws:route53:::hostedzone/Z1NBZRVGMI91Y9",
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

resource "aws_iam_user_policy_attachment" "monks-go_iam_user_write_ss_cx_records" {
  user       = aws_iam_user.monks-go.name
  policy_arn = aws_iam_policy.write_ss_cx_records.arn
}

