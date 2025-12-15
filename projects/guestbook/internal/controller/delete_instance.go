package controller

import (
	"context"
	"time"

	computev1 "cloud.com/guestbook/api/v1"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func deleteInstance(ctx context.Context, ec2Instance *computev1.EC2instance) (bool, error) {
		l := log.FromContext(ctx)

		l.Info("=== Deleting the instance ===", "instanceID", ec2Instance.Status.InstanceID)

		ec2Client := awsClient(ec2Instance.Spec.Region)

		terminateResult, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: []string{ec2Instance.Status.InstanceID},
		})

		if err != nil {
			l.Error(err, "Failed to terminate instance")
			return false, err
		}

		l.Info("Instance termination initiated",
	           "instanceID",ec2Instance.Status.InstanceID,
			   "currentState", terminateResult.TerminatingInstances[0].CurrentState.Name)
		
		waiter := ec2.NewInstanceTerminatedWaiter(ec2Client)

		maxWaitTime := 5 * time.Minute
		waitParams := &ec2.DescribeInstancesInput{
			InstanceIds: []string{ec2Instance.Status.InstanceID},
		}

		l.Info("Waiting for instance to be terminated",
	            "instanceID",ec2Instance.Status.InstanceID,
			    "maxWaitTime",maxWaitTime)

		err = waiter.Wait(ctx, waitParams, maxWaitTime)

		if err != nil {
			l.Error(err, "Failed while waiting for instance termination", "instanceID", ec2Instance.Status.InstanceID)
			return false, err
		}
        

		l.Info("=== EC2 instance deleted successfully ===", "instanceID", ec2Instance.Status.InstanceID)

		return true, nil

}