package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	egoscale "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
)

const providerName = "exoscale"

var (
	recordTTL = 60
)

// ExoscaleSolver implements the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface and allows cert-manager to use [exoscale DNS] for DNS01 challenge.
//
// [exoscale DNS]: https://community.exoscale.com/documentation/dns/
type ExoscaleSolver struct {
	kClient *kubernetes.Clientset
	logger  Logger
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
func (c *ExoscaleSolver) Name() string {
	return providerName
}

// Initialize will be called when the webhook first starts.
// The kubeClientConfig parameter is used to build a Kubernetes
// client that will be used to fetch Secret resources containing
// credentials used to authenticate with Exoscale API.
func (c *ExoscaleSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	c.logger = Logger{}
	// debug can be set only through environment.
	if os.Getenv(envDebug) != "" {
		c.logger.Verbose = true
	}

	c.logger.Debug("call function Initialize")

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.kClient = cl

	return nil
}

// Present is responsible for actually presenting the DNS record with the
// Exoscale DNS.
func (c *ExoscaleSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	c.logger.Debugf("call function Present: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := LoadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := c.apiClient(ch, config)
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	ctx := context.Background()

	domain, err := c.findDomain(ctx, client, strings.TrimSuffix(ch.ResolvedZone, "."))
	if err != nil {
		return err
	}

	recordName := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	t := int64(recordTTL)
	recordRequest := egoscale.CreateDNSDomainRecordRequest{
		Name:    recordName,
		Type:    egoscale.CreateDNSDomainRecordRequestTypeTXT,
		Ttl:     t,
		Content: ch.Key,
	}

	op, err := client.CreateDNSDomainRecord(ctx, domain.ID, recordRequest)
	if err != nil {
		return fmt.Errorf("exoscale: error while creating DNS record: %w", err)
	}

	_, err = client.Wait(ctx, op, egoscale.OperationStateSuccess)
	if err != nil {
		return fmt.Errorf("exoscale: error while waiting for DNS record creation: %w", err)
	}

	return nil
}

// CleanUp deletes the relevant TXT record from the Exoscale DNS.
func (c *ExoscaleSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	c.logger.Debugf("call function CleanUp: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := LoadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := c.apiClient(ch, config)
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	ctx := context.Background()

	domain, err := c.findDomain(ctx, client, strings.TrimSuffix(ch.ResolvedZone, "."))
	if err != nil {
		return err
	}

	records, err := client.ListDNSDomainRecords(ctx, domain.ID)
	if err != nil {
		return err
	}

	recordName := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	for _, record := range records.DNSDomainRecords {
		// we must unquote TXT records as we receive "\"123d==\"" when we expect "123d=="
		content, _ := strconv.Unquote(record.Content)
		if record.Type == egoscale.DNSDomainRecordTypeTXT &&
			record.Name == recordName &&
			content == ch.Key {

			op, err := client.DeleteDNSDomainRecord(ctx, domain.ID, record.ID)
			if err != nil {
				return fmt.Errorf("exoscale: error while deleting DNS record: %w", err)
			}

			_, err = client.Wait(ctx, op, egoscale.OperationStateSuccess)
			if err != nil {
				return fmt.Errorf("exoscale: error while waiting DNS record deletion: %w", err)
			}

			break
		}
	}

	c.logger.Infof("domain record %q not found, nothing to do", recordName)

	return nil
}

// findDomain is a helper that iterates through domain list to find a domain by name.
// This is needed as API can only query by ID (and not name).
// Returns error if domain is not found.
func (c *ExoscaleSolver) findDomain(
	ctx context.Context,
	client *egoscale.Client,
	domainName string,
) (*egoscale.DNSDomain, error) {
	domains, err := client.ListDNSDomains(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving DNS domain list: %w", err)
	}

	for _, domain := range domains.DNSDomains {
		if domain.UnicodeName == domainName {
			return &domain, nil
		}
	}

	return nil, fmt.Errorf("domain %q not found", domainName)
}

// apiClient is a helper that initializes Egoscale (Exoscale API) client.
// Resolves any configuration overrides from environment.
func (c *ExoscaleSolver) apiClient(ch *v1alpha1.ChallengeRequest, config Config) (*egoscale.Client, error) {
	var apiKey, apiSecret string

	var opts []egoscale.ClientOpt

	switch {
	case os.Getenv(envAPIKey) != "" && os.Getenv(envAPISecret) != "":
		// env always take precedence over config.
		c.logger.Info("found client credentials in environment, ignoring config")
		apiKey = os.Getenv(envAPIKey)
		apiSecret = os.Getenv(envAPISecret)
	case config.APIKeyRef != nil && config.APISecretRef != nil:
		apiKeyResource, err := c.kClient.CoreV1().Secrets(ch.ResourceNamespace).Get(
			context.Background(),
			config.APIKeyRef.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("could not get secret %s: %w", config.APIKeyRef.Name, err)
		}

		apiSecretResource, err := c.kClient.CoreV1().Secrets(ch.ResourceNamespace).Get(
			context.Background(),
			config.APISecretRef.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("could not get secret %s: %w", config.APISecretRef.Name, err)
		}

		apiKeyData, ok := apiKeyResource.Data[config.APIKeyRef.Key]
		if !ok {
			return nil, fmt.Errorf("could not get key %s in secret %s", config.APIKeyRef.Key, config.APIKeyRef.Name)
		}

		apiSecretData, ok := apiSecretResource.Data[config.APISecretRef.Key]
		if !ok {
			return nil, fmt.Errorf("could not get key %s in secret %s", config.APISecretRef.Key, config.APISecretRef.Name)
		}

		apiKey = string(apiKeyData)
		apiSecret = string(apiSecretData)
	default:
		return nil, errors.New("client credentials not found")
	}

	// Add User-Agent
	opts = append(opts, egoscale.ClientOptWithUserAgent("cert-manager-webhook-exoscale/"+Version))

	// Check the TRACE environment variable
	// API trace mode can be set only through environment.
	if v := os.Getenv(envTrace); v != "" {
		opts = append(opts, egoscale.ClientOptWithTrace())
	}

	return egoscale.NewClient(
		credentials.NewStaticCredentials(apiKey, apiSecret),
		opts...,
	)
}
