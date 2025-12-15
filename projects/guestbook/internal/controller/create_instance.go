package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	//"sigs.k8s.io/controller-runtime"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "cloud.com/guestbook/api/v1"
)

func createEc2Instance(ec2Instance *computev1.EC2instance) (createdInstanceInfo *computev1.CreatedInstanceInfo, err error) {
    l := log.Log.WithName("createEc2Instance")

	l.Info("=== STARTING EC2 INSTANCE CREATION ===",
		"ami", ec2Instance.Spec.AMIId,
		"instanceType", ec2Instance.Spec.InstanceType,
		"region", ec2Instance.Spec.Region)
    
    
    
    ec2Client := awsClient(ec2Instance.Spec.Region)

	// create the input for the run instances
	runInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(ec2Instance.Spec.AMIId),
		InstanceType: ec2types.InstanceType(ec2Instance.Spec.InstanceType),
		KeyName:      aws.String(ec2Instance.Spec.KeyPair),
		SubnetId:     aws.String(ec2Instance.Spec.Subnet),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		//SecurityGroupIds: []string{ec2Instance.Spec.SecurityGroups[0]},
	}

	l.Info("==== Calling AWS Runinstances API ====")
	result, err := ec2Client.RunInstances(context.TODO(), runInput)
	if err != nil {
		l.Error(err, "Failed to create EC2 instance")
		return nil, fmt.Errorf("failed to create EC2 instance: %w", err)
	}

	if len(result.Instances) == 0 {
		l.Error(nil, "No instances returned in RunInstancesOutput")
		fmt.Println("No instances returned in RunInstancesOutput")
		return nil, nil
	}

	inst := result.Instances[0]
	l.Info("=== EC2 instance created successfully ===","instanceID", *inst.InstanceId)

	l.Info("=== Waiting for the instance to be running ===")

	runWaiter := ec2.NewInstanceRunningWaiter(ec2Client)
	maxWaitTime := 3 * time.Minute
    
	// Waiting for the instance to running
	err = runWaiter.Wait(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}, maxWaitTime)
	if err != nil {
		l.Error(err, "Failed to wait for the instance to be running")
		return nil, fmt.Errorf("Failed to wait for the instance to be running: %w", err)
	}

	//After instance is in running state, get the instance details
	l.Info("=== Calling AWS DescribeInstances API to get instance details ===")
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}

	describeResult, err := ec2Client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		l.Error(err, "Failed to describe EC2 instance")
		return nil, fmt.Errorf("Failed to describe EC2 instance: %w", err)
	}

	fmt.Println("Describe result", "public ip", describeResult.Reservations[0].Instances[0].PublicDnsName, "state", describeResult.Reservations[0].Instances[0].State.Name)
	fmt.Printf("Private IP of the instance: %v", derefString(inst.PrivateIpAddress))
    fmt.Printf("State of the instance: %v", describeResult.Reservations[0].Instances[0].State.Name)
	fmt.Printf("Private DNS of the instance: %v", derefString(inst.PrivateDnsName))
	fmt.Printf("Instance ID of the instance: %v", derefString(inst.InstanceId))
	fmt.Println("Instance Type of the instance: ", inst.InstanceType)
	fmt.Printf("Image ID of the instance: %v", derefString(inst.ImageId))
	fmt.Printf("Key Name of the instance: %v", derefString(inst.KeyName))

	instance := describeResult.Reservations[0].Instances[0]
	createdInstanceInfo = &computev1.CreatedInstanceInfo{
		InstanceID: *inst.InstanceId,
		State:      string(instance.State.Name),
		PublicIP:   derefString(instance.PublicIpAddress),
		PrivateIP:  derefString(instance.PrivateIpAddress),
		PublicDNS:  derefString(instance.PublicDnsName),
		PrivateDNS: derefString(instance.PrivateDnsName),
	}

	l.Info("=== EC2 INSTANCE CREATION COMPLETED ===",
		"instanceID", createdInstanceInfo.InstanceID,
		"state", createdInstanceInfo.State,
		"publicIP", createdInstanceInfo.PublicIP)

	// For now, just return nil to indicate success.
	return createdInstanceInfo, nil

}

// derefString is a helper function to safely dereference *string
func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return "<nil>"
}

