resource "aws_iam_user" "user" {
  name = "mailer-${var.domain}"
}

resource "aws_iam_access_key" "access_key" {
  user = aws_iam_user.user.name
}

data "aws_iam_policy_document" "policy_document" {
  statement {
    actions   = ["ses:SendEmail", "ses:SendRawEmail"]
    resources = [aws_ses_domain_identity.mailer.arn]
  }
}

resource "aws_iam_policy" "policy" {
  name   = "Mailer-${var.domain}"
  policy = data.aws_iam_policy_document.policy_document.json
}

resource "aws_iam_user_policy_attachment" "user_policy" {
  user       = aws_iam_user.user.name
  policy_arn = aws_iam_policy.policy.arn
}

