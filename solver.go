package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	exoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"
)

const providerName = "exoscale"

var (
	recordTypeTXT = "TXT"
	recordTTL     = 60
)

// ExoscaleSolver implements the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface and allows cert-manager to use [exoscale DNS] for DNS01 challenge.
//
// [exoscale DNS]: https://community.exoscale.com/documentation/dns/
type ExoscaleSolver struct {
	kClient *kubernetes.Clientset
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
	log.Println("call function Initialize")

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
	log.Printf("call function Present: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := LoadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := c.apiClient(ch, config)
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	ctx := exoapi.WithEndpoint(
		context.Background(),
		exoapi.NewReqEndpoint(config.APIEnvironment, config.APIZone),
	)

	domain, err := c.findDomain(ctx, client, config.APIZone, strings.TrimSuffix(ch.ResolvedZone, "."))
	if err != nil {
		return err
	}

	recordName := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	t := int64(recordTTL)
	record := exoscale.DNSDomainRecord{
		Name:    &recordName,
		Type:    &recordTypeTXT,
		TTL:     &t,
		Content: &ch.Key,
	}

	_, err = client.CreateDNSDomainRecord(ctx, config.APIZone, *domain.ID, &record)
	if err != nil {
		return fmt.Errorf("failed to create domain record: %w", err)
	}

	return nil
}

// CleanUp deletes the relevant TXT record from the Exoscale DNS.
func (c *ExoscaleSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	log.Printf("call function CleanUp: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := LoadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := c.apiClient(ch, config)
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	ctx := exoapi.WithEndpoint(
		context.Background(),
		exoapi.NewReqEndpoint(config.APIEnvironment, config.APIZone),
	)

	domain, err := c.findDomain(ctx, client, config.APIZone, strings.TrimSuffix(ch.ResolvedZone, "."))
	if err != nil {
		return err
	}

	records, err := client.ListDNSDomainRecords(ctx, config.APIZone, *domain.ID)
	if err != nil {
		return err
	}

	recordName := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	for _, record := range records {
		if *record.Type == recordTypeTXT &&
			*record.Name == recordName &&
			*record.Content == ch.Key {
			return client.DeleteDNSDomainRecord(ctx, config.APIZone, *domain.ID, &record)
		}
	}

	log.Printf("domain record %q not found, nothing to do", recordName)

	return nil
}

// findDomain is a helper that iterates through domain list to find a domain by name.
// This is needed as API can only query by ID (and not name).
// Returns error if domain is not found.
func (c *ExoscaleSolver) findDomain(
	ctx context.Context,
	client *exoscale.Client,
	apiZone string,
	domainName string,
) (*exoscale.DNSDomain, error) {
	domains, err := client.ListDNSDomains(ctx, apiZone)
	if err != nil {
		return nil, fmt.Errorf("error retrieving domain list: %w", err)
	}

	for _, domain := range domains {
		if *domain.UnicodeName == domainName {
			return &domain, nil
		}
	}

	return nil, fmt.Errorf("domain %q not found", domainName)
}

// apiClient is a helper that initializes Egoscale (Exoscale API) client,
// as well as configuring a proper context.
func (c *ExoscaleSolver) apiClient(ch *v1alpha1.ChallengeRequest, config Config) (*exoscale.Client, error) {
	var apiKey, apiSecret string

	switch {
	case os.Getenv(envAPIKey) != "" && os.Getenv(envAPISecret) != "":
		// env always take precedence over config.
		log.Println("found client credentials in environment, ignoring config")
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

	return exoscale.NewClient(
		apiKey,
		apiSecret,
		// API trace mode can be set only through environment.
		exoscale.ClientOptCond(func() bool {
			if v := os.Getenv(envTrace); v != "" {
				return true
			}
			return false
		}, exoscale.ClientOptWithTrace()),
	)
}
