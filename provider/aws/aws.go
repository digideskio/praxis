package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/simpledb"
	"github.com/fsouza/go-dockerclient"
)

const (
	sortableTime = "20060102.150405.000000000"
)

type Provider struct {
	Config      *aws.Config
	Development bool
	Rack        string
	Region      string
	Session     *session.Session
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func FromEnv() (*Provider, error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	region := os.Getenv("AWS_REGION")

	return &Provider{
		Config:      &aws.Config{Region: aws.String(region)},
		Development: os.Getenv("DEVELOPMENT") == "true",
		Rack:        os.Getenv("RACK"),
		Region:      region,
		Session:     session,
	}, nil
}

func (p *Provider) CloudFormation() *cloudformation.CloudFormation {
	return cloudformation.New(p.Session, p.Config)
}

func (p *Provider) CloudWatch() *cloudwatch.CloudWatch {
	return cloudwatch.New(p.Session, p.Config)
}

func (p *Provider) CloudWatchLogs() *cloudwatchlogs.CloudWatchLogs {
	return cloudwatchlogs.New(p.Session, p.Config)
}

func (p *Provider) Docker(host string) (*docker.Client, error) {
	return docker.NewClient(host)
}

func (p *Provider) ECS() *ecs.ECS {
	return ecs.New(p.Session, p.Config)
}

func (p *Provider) EC2() *ec2.EC2 {
	return ec2.New(p.Session, p.Config)
}

func (p *Provider) IAM() *iam.IAM {
	return iam.New(p.Session, p.Config)
}

func (p *Provider) S3() *s3.S3 {
	return s3.New(p.Session, p.Config)
}

func (p *Provider) SimpleDB() *simpledb.SimpleDB {
	return simpledb.New(p.Session, p.Config)
}

func awsError(err error) string {
	if ae, ok := err.(awserr.Error); ok {
		return ae.Code()
	}
	return ""
}

func formationTemplate(name string, data interface{}) ([]byte, error) {
	var buf bytes.Buffer

	tn := fmt.Sprintf("%s.json.tmpl", name)
	tf := fmt.Sprintf("provider/aws/formation/%s", tn)

	t, err := template.New(tn).Funcs(formationHelpers()).ParseFiles(tf)
	if err != nil {
		return nil, err
	}

	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}

	var v interface{}

	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		return nil, err
	}

	return json.MarshalIndent(v, "", "  ")
}

func formationHelpers() template.FuncMap {
	return template.FuncMap{
		"resource": func(s string) string {
			fmt.Printf("s = %+v\n", s)
			return s
		},
	}
}

func (p *Provider) appRepository(app string) (string, error) {
	res, err := p.IAM().GetUser(&iam.GetUserInput{})
	if err != nil {
		return "", err
	}

	parts := strings.Split(*res.User.Arn, ":")

	if len(parts) != 6 {
		return "", fmt.Errorf("invalid user arn")
	}

	aid := parts[4]

	repo, err := p.appResource(app, "Repository")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", aid, p.Region, repo), nil
}

func (p *Provider) appResource(app string, resource string) (string, error) {
	return p.stackResource(fmt.Sprintf("%s-%s", p.Rack, app), resource)
}

func (p *Provider) rackResource(resource string) (string, error) {
	return p.stackResource(p.Rack, resource)
}

func (p *Provider) stackResource(name string, resource string) (string, error) {
	res, err := p.CloudFormation().DescribeStackResource(&cloudformation.DescribeStackResourceInput{
		LogicalResourceId: aws.String(resource),
		StackName:         aws.String(name),
	})
	if err != nil {
		return "", err
	}

	return *res.StackResourceDetail.PhysicalResourceId, nil
}
