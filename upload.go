package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

type GraphAuth struct {
	CLIENT_ID     string `yaml:"CLIENT_ID"`
	CLIENT_SECRET string `yaml:"CLIENT_SECRET"`
	TENANT_ID     string `yaml:"TENANT_ID"`
	SITE          string `yaml:"site"`
	SITE_ID       string `yaml:"site_id"`
}

type GraphHelper struct {
	clientSecretCredential *azidentity.ClientSecretCredential
	appClient              *msgraphsdk.GraphServiceClient
	token                  *string
}

type UploadSplit struct {
	length *string
	ranges *string
}

type UploadSession struct {
	Context            string
	ExpirationDateTime string
	NextExpectedRanges []string
	UploadURL          string
}

func (ga GraphAuth) NewGraphHelper() *GraphHelper {
	g := &GraphHelper{}
	err := g.InitializeGraphForAppAuth(ga)
	if err != nil {
		log.Panicf("Error initializing Graph for app auth: %v\n", err)
	}

	return g
}

func (g *GraphHelper) InitializeGraphForAppAuth(ga GraphAuth) error {
	clientID := ga.CLIENT_ID
	tenantID := ga.TENANT_ID
	clientSecret := ga.CLIENT_SECRET
	credential, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return err
	}

	g.clientSecretCredential = credential

	// Create an auth provider using the credential
	authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(g.clientSecretCredential, []string{
		"https://graph.microsoft.com/.default",
	})
	if err != nil {
		return err
	}

	// Create a request adapter using the auth provider
	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return err
	}

	// Create a Graph client using request adapter
	client := msgraphsdk.NewGraphServiceClient(adapter)
	g.appClient = client
	g.GetAppToken()
	return nil
}

func (g *GraphHelper) GetUsers() (models.UserCollectionResponseable, error) {
	var topValue int32 = 25
	query := users.UsersRequestBuilderGetQueryParameters{
		// Only request specific properties
		Select: []string{"displayName", "id", "mail"},
		// Get at most 25 results
		Top: &topValue,
		// Sort by display name
		Orderby: []string{"displayName"},
	}

	return g.appClient.Users().
		Get(context.Background(),
			&users.UsersRequestBuilderGetRequestConfiguration{
				QueryParameters: &query,
			})
}

func (g *GraphHelper) GetAppToken() {
	token, err := g.clientSecretCredential.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{
			"https://graph.microsoft.com/.default",
		},
	})
	if err != nil {
		log.Printf("Error getting user token: %v\n", err)
	}
	g.token = &token.Token
}

func (g *GraphHelper) createSession(site string, siteID string, name string) {
	var buffer []byte
	header := make(map[string]string)
	header["Authorization"] = "Bearer " + *g.token
	url := "https://graph.microsoft.com/v1.0/" + site + "/" + siteID + "/root:/" + name + ":/createUploadSession"
	js, err := makeRequest("POST", url, header, buffer)
	if err != nil {
		log.Println(err)
	}
	var jsg UploadSession
	err = json.Unmarshal(js, &jsg)
	if err != nil {
		log.Fatal(err)
	}

	chunkSize := int64(327680)
	uploadFile(jsg.UploadURL, name, chunkSize)
	fmt.Println("upload success")
}

func makeRequest(method string, url string, header map[string]string, body []byte) ([]byte, error) {
	var uploadSession []byte = nil
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
		return uploadSession, err
	}
	for key, value := range header {
		req.Header.Add(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return uploadSession, err
	}
	defer resp.Body.Close()
	sitemap, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		err = json.Unmarshal(sitemap, &uploadSession)
		if err != nil {
			log.Fatal(err)
		}
		return uploadSession, err
	}
	uploadSession = sitemap
	return uploadSession, nil
}

func getFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func uploadFile(url string, inputPath string, chunkSize int64) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()
	fileSize, err := getFileSize(inputPath)
	buffer := make([]byte, chunkSize)
	for i := int64(0); i < fileSize; i += int64(chunkSize) {
		n, err := inputFile.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		l := strconv.Itoa(n)
		r := "bytes " + strconv.FormatInt(i, 10) + "-" + strconv.FormatInt(i+int64(n)-1, 10) + "/" + strconv.FormatInt(fileSize, 10)
		header := make(map[string]string)
		header["Content-Length"] = l
		header["Content-Range"] = r
		_, err = makeRequest("PUT", url, header, buffer[:n])
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println(inputPath, i/chunkSize, "/", fileSize/chunkSize)
		}
	}
	return nil
}

func (g *GraphHelper) StartUpdate(site string, siteID string, file string) {
	g.createSession(site, siteID, file)
}
