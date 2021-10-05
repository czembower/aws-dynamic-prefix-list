package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type LambdaEvent struct {
	Name string `json:"name"`
}

func getCurrentCidrs() []string {
	var saasCidrs []byte
	var onpremCidrs []byte
	var onpremCidrsList []string
	var saasCidrsList []string

	token := os.Getenv("API_TOKEN") // if a token is required, get from env vars

	resp, err := http.Get("https://listofcidrs.net/permitlistSaasS?token=" + token)

	if resp.StatusCode == 200 {
		if err == nil {
			saasCidrs, _ = ioutil.ReadAll(resp.Body)
			saasCidrsList = strings.Split(string(saasCidrs), "\n")
		} else {
			panic(err)
		}
	} else {
		panic("Access to API resource unsuccessful.")
	}

	resp, err = http.Get("https:/listofcidrs.net/permitlistOnPrem?token=" + token)

	if resp.StatusCode == 200 {
		if err == nil {
			onpremCidrs, _ = ioutil.ReadAll(resp.Body)
			onpremCidrsList = strings.Split(string(onpremCidrs), "\n")
		} else {
			panic(err)
		}
	} else {
		panic("Access to API resource unsuccessful.")
	}

	cidrsList := append(saasCidrsList, onpremCidrsList...)

	return cidrsList
}

func modifyPl(currentCidrs []string, plId string, plVersion int64, plExistingEntries []types.PrefixListEntry) {
	var plEntries []types.AddPrefixListEntry
	var plEntriesToRemove []types.RemovePrefixListEntry

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	client := ec2.NewFromConfig(cfg)

	if len(plExistingEntries) != 0 {

		for _, p := range plExistingEntries {
			plEntriesToRemove = append(plEntriesToRemove, types.RemovePrefixListEntry{Cidr: aws.String(*p.Cidr)})
		}
		params := ec2.ModifyManagedPrefixListInput{
			PrefixListId:   aws.String(plId),
			RemoveEntries:  plEntriesToRemove,
			CurrentVersion: aws.Int64(plVersion),
		}

		for {
			out, err := client.ModifyManagedPrefixList(context.TODO(), &params)

			if err == nil {
				println("Removed existing prefixes for", *out.PrefixList.PrefixListName)
				break
			} else {
				time.Sleep(1 * time.Second)
			}
		}
		plVersion++
	}

	for _, p := range currentCidrs {
		entry := types.AddPrefixListEntry{Cidr: aws.String(p)}
		plEntries = append(plEntries, entry)
		fmt.Printf("Added CIDR: %v\n", string(*entry.Cidr))
	}

	params := ec2.ModifyManagedPrefixListInput{
		PrefixListId:   aws.String(plId),
		AddEntries:     plEntries,
		CurrentVersion: aws.Int64(plVersion),
	}

	for {
		out, err := client.ModifyManagedPrefixList(context.TODO(), &params)

		if err == nil {
			println("Modified", *out.PrefixList.PrefixListName)
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

func createPl() string {
	var plId string
	var plEntries []types.AddPrefixListEntry

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	client := ec2.NewFromConfig(cfg)

	params := ec2.CreateManagedPrefixListInput{
		PrefixListName: aws.String("myDynamicPrefixList"),
		AddressFamily:  aws.String("ipv4"),
		MaxEntries:     aws.Int32(64),
		Entries:        plEntries,
	}

	out, err := client.CreateManagedPrefixList(context.TODO(), &params)

	if err != nil {
		panic(err)
	} else {
		plId = *out.PrefixList.PrefixListId
		fmt.Println("Created Managed Prefix List: ", plId)
	}

	return plId
}

func getPl() (string, int64, []types.PrefixListEntry) {
	var plId string
	var plVersion int64
	var plExistingEntries *ec2.GetManagedPrefixListEntriesOutput

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	client := ec2.NewFromConfig(cfg)

	filters :=
		[]types.Filter{
			{
				Name: aws.String("prefix-list-name"),
				Values: []string{
					"myDynamicPrefixList",
				},
			},
		}

	params := ec2.DescribeManagedPrefixListsInput{
		Filters: filters,
	}

	out, err := client.DescribeManagedPrefixLists(context.TODO(), &params)

	if err != nil {
		panic(err)
	} else {
		if len(out.PrefixLists) == 0 {
			plId = createPl()
		} else {
			for _, p := range out.PrefixLists {
				plId = *p.PrefixListId
				plVersion = *p.Version
				fmt.Println("Found Prefix List with ID: ", plId)
				paramsEntries := ec2.GetManagedPrefixListEntriesInput{
					PrefixListId: aws.String(plId),
				}
				plExistingEntries, err = client.GetManagedPrefixListEntries(context.TODO(), &paramsEntries)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	return plId, plVersion, plExistingEntries.Entries
}

func HandleRequest(ctx context.Context, name LambdaEvent) {
	currentCidrs := getCurrentCidrs()
	plId, plVersion, plEntries := getPl()
	modifyPl(currentCidrs, plId, plVersion, plEntries)
}

func main() {
	lambda.Start(HandleRequest)
}
