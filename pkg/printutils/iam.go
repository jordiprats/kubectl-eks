package printutils

import (
	"fmt"
	"os"
	"sort"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintIRSA(noHeaders bool, irsaInfos ...data.IRSAInfo) {
	sort.Slice(irsaInfos, func(i, j int) bool {
		if irsaInfos[i].Profile != irsaInfos[j].Profile {
			return irsaInfos[i].Profile < irsaInfos[j].Profile
		}
		if irsaInfos[i].Region != irsaInfos[j].Region {
			return irsaInfos[i].Region < irsaInfos[j].Region
		}
		if irsaInfos[i].ClusterName != irsaInfos[j].ClusterName {
			return irsaInfos[i].ClusterName < irsaInfos[j].ClusterName
		}
		if irsaInfos[i].Namespace != irsaInfos[j].Namespace {
			return irsaInfos[i].Namespace < irsaInfos[j].Namespace
		}
		return irsaInfos[i].ServiceAccountName < irsaInfos[j].ServiceAccountName
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "SERVICE ACCOUNT", Type: "string"},
			{Name: "IAM ROLE ARN", Type: "string"},
		},
	}

	for _, info := range irsaInfos {
		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				info.Profile,
				info.Region,
				info.ClusterName,
				info.Namespace,
				info.ServiceAccountName,
				info.IAMRoleARN,
			},
		})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintKube2IAM(noHeaders bool, kube2iamInfos ...data.Kube2IAMInfo) {
	multiCluster := false
	if len(kube2iamInfos) > 0 {
		first := kube2iamInfos[0].ClusterName
		for _, info := range kube2iamInfos[1:] {
			if info.ClusterName != first {
				multiCluster = true
				break
			}
		}
	}

	sort.Slice(kube2iamInfos, func(i, j int) bool {
		if kube2iamInfos[i].Profile != kube2iamInfos[j].Profile {
			return kube2iamInfos[i].Profile < kube2iamInfos[j].Profile
		}
		if kube2iamInfos[i].Region != kube2iamInfos[j].Region {
			return kube2iamInfos[i].Region < kube2iamInfos[j].Region
		}
		if kube2iamInfos[i].ClusterName != kube2iamInfos[j].ClusterName {
			return kube2iamInfos[i].ClusterName < kube2iamInfos[j].ClusterName
		}
		if kube2iamInfos[i].Namespace != kube2iamInfos[j].Namespace {
			return kube2iamInfos[i].Namespace < kube2iamInfos[j].Namespace
		}
		return kube2iamInfos[i].PodName < kube2iamInfos[j].PodName
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	columns := []v1.TableColumnDefinition{}
	if multiCluster {
		columns = append(columns,
			v1.TableColumnDefinition{Name: "AWS PROFILE", Type: "string"},
			v1.TableColumnDefinition{Name: "AWS REGION", Type: "string"},
			v1.TableColumnDefinition{Name: "CLUSTER", Type: "string"},
		)
	}
	columns = append(columns,
		v1.TableColumnDefinition{Name: "NAMESPACE", Type: "string"},
		v1.TableColumnDefinition{Name: "POD", Type: "string"},
		v1.TableColumnDefinition{Name: "IAM ROLE", Type: "string"},
		v1.TableColumnDefinition{Name: "NODE", Type: "string"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	for _, info := range kube2iamInfos {
		cells := []interface{}{}
		if multiCluster {
			cells = append(cells, info.Profile, info.Region, info.ClusterName)
		}
		cells = append(cells, info.Namespace, info.PodName, info.IAMRole, info.NodeName)
		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintPodIdentity(noHeaders bool, podIdentityInfos ...data.PodIdentityInfo) {
	sort.Slice(podIdentityInfos, func(i, j int) bool {
		if podIdentityInfos[i].Profile != podIdentityInfos[j].Profile {
			return podIdentityInfos[i].Profile < podIdentityInfos[j].Profile
		}
		if podIdentityInfos[i].Region != podIdentityInfos[j].Region {
			return podIdentityInfos[i].Region < podIdentityInfos[j].Region
		}
		if podIdentityInfos[i].ClusterName != podIdentityInfos[j].ClusterName {
			return podIdentityInfos[i].ClusterName < podIdentityInfos[j].ClusterName
		}
		if podIdentityInfos[i].Namespace != podIdentityInfos[j].Namespace {
			return podIdentityInfos[i].Namespace < podIdentityInfos[j].Namespace
		}
		return podIdentityInfos[i].ServiceAccountName < podIdentityInfos[j].ServiceAccountName
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "SERVICE ACCOUNT", Type: "string"},
			{Name: "IAM ROLE ARN", Type: "string"},
			{Name: "TYPE", Type: "string"},
		},
	}

	for _, info := range podIdentityInfos {
		roleArn := info.IAMRoleARN
		if roleArn == "" {
			roleArn = "<not set>"
		}

		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				info.Profile,
				info.Region,
				info.ClusterName,
				info.Namespace,
				info.ServiceAccountName,
				roleArn,
				info.IdentityType,
			},
		})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}
