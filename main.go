package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// rmreq = remove request
type rmreq struct {
	id     string
	region string
	svc    *ec2.EC2
	vpc_id string
}

func getRegions(sess *session.Session) []*ec2.Region {
	client := ec2.New(sess)
	result, err := client.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		panic(err)
	}

	return result.Regions
}

func main() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	}))
	var wg sync.WaitGroup

	for _, region := range getRegions(sess) {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()

			rsess := session.Must(session.NewSession(&aws.Config{
				Region: aws.String(region),
			}))

			svc := ec2.New(rsess)
			attrs, err := svc.DescribeAccountAttributes(
				&ec2.DescribeAccountAttributesInput{
					AttributeNames: []*string{
						aws.String("default-vpc"),
					},
				},
			)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"error: %s account attrs: %s\n",
					region,
					err.Error(),
				)
				return
			}

			vpc_id := aws.StringValue(
				attrs.AccountAttributes[0].AttributeValues[0].AttributeValue,
			)

			rmIgws(vpc_id, region, svc)
			rmSubns(vpc_id, region, svc)
			rmRtbs(vpc_id, region, svc)
			rmAcls(vpc_id, region, svc)
			rmSgs(vpc_id, region, svc)
			rmVpc(vpc_id, region, svc)
		}(aws.StringValue(region.RegionName))
	}

	wg.Wait()
}

func makeEC2Filters(attr, val string) []*ec2.Filter {
	return []*ec2.Filter{
		{
			Name: aws.String(attr),
			Values: []*string{
				aws.String(val),
			},
		},
	}
}

func rmAcls(vpc_id, region string, svc *ec2.EC2) {
	result, err := svc.DescribeNetworkAcls(&ec2.DescribeNetworkAclsInput{
		Filters: makeEC2Filters("vpc-id", vpc_id),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %s %s failed to get nacls: %s\n",
			region,
			vpc_id,
			err.Error(),
		)
		return
	}

	for _, acl := range result.NetworkAcls {
		_, err := svc.DeleteNetworkAcl(&ec2.DeleteNetworkAclInput{
			NetworkAclId: acl.NetworkAclId,
		})
		errmsg := ""
		if err != nil {
			errmsg = fmt.Sprintf(" (error: %s)", err.Error())
		}
		fmt.Printf("rm %-15s %-15s%s\n",
			region,
			aws.StringValue(acl.NetworkAclId),
			errmsg,
		)
	}
}

func rmIgws(vpc_id, region string, svc *ec2.EC2) {
	result, err := svc.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
		Filters: makeEC2Filters("attachment.vpc-id", vpc_id),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %s %s failed to get igws: %s\n",
			region,
			vpc_id,
			err.Error(),
		)
		return
	}

	for _, igw := range result.InternetGateways {
		errmsg := ""

		_, err := svc.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
			VpcId:             aws.String(vpc_id),
		})
		if err != nil {
			errmsg = fmt.Sprintf(" (detach error: %s)", err.Error())
		}

		if err == nil {
			_, err = svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
				InternetGatewayId: igw.InternetGatewayId,
			})
			if err != nil {
				errmsg = fmt.Sprintf(" (error: %s)", err.Error())
			}
		}

		fmt.Printf("rm %-15s %-15s%s\n",
			region,
			aws.StringValue(igw.InternetGatewayId),
			errmsg,
		)
	}
}

func rmRtbs(vpc_id, region string, svc *ec2.EC2) {
	result, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
		Filters: makeEC2Filters("vpc-id", vpc_id),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %s %s failed to get rtbs: %s\n",
			region,
			vpc_id,
			err.Error(),
		)
		return
	}

	for _, rtb := range result.RouteTables {
		_, err := svc.DeleteRouteTable(&ec2.DeleteRouteTableInput{
			RouteTableId: rtb.RouteTableId,
		})
		errmsg := ""
		if err != nil {
			errmsg = fmt.Sprintf(" (error: %s)", err.Error())
		}
		fmt.Printf("rm %-15s %-15s%s\n",
			region,
			aws.StringValue(rtb.RouteTableId),
			errmsg,
		)
	}
}

func rmSgs(vpc_id, region string, svc *ec2.EC2) {
	result, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: makeEC2Filters("vpc-id", vpc_id),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %s %s failed to get sgs: %s\n",
			region,
			vpc_id,
			err.Error(),
		)
		return
	}

	for _, sg := range result.SecurityGroups {
		_, err := svc.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		})
		errmsg := ""
		if err != nil {
			errmsg = fmt.Sprintf(" (error: %s)", err.Error())
		}
		fmt.Printf("rm %-15s %-15s%s\n",
			region,
			aws.StringValue(sg.GroupId),
			errmsg,
		)
	}
}

func rmSubns(vpc_id, region string, svc *ec2.EC2) {
	result, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: makeEC2Filters("vpc-id", vpc_id),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %s %s failed to get subnets: %s\n",
			region,
			vpc_id,
			err.Error(),
		)
		return
	}

	for _, subnet := range result.Subnets {
		_, err := svc.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		errmsg := ""
		if err != nil {
			errmsg = fmt.Sprintf(" (error: %s)", err.Error())
		}
		fmt.Printf("rm %-15s %-15s%s\n",
			region,
			aws.StringValue(subnet.SubnetId),
			errmsg,
		)
	}
}

func rmVpc(vpc_id, region string, svc *ec2.EC2) {
	_, err := svc.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: aws.String(vpc_id),
	})
	errmsg := ""
	if err != nil {
		errmsg = fmt.Sprintf(" (error: %s)", err.Error())
	}
	fmt.Printf("rm %-15s %-15s%s\n", region, vpc_id, errmsg)
}
