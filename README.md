# Azure App Configuration Kubernetes Provider

[Azure App Configuration Kubernetes Provider](https://mcr.microsoft.com/product/azure-app-configuration/kubernetes-provider/about) can construct ConfigMaps and Secrets from key-values and Key Vault references in [Azure App Configuration](https://learn.microsoft.com/azure/azure-app-configuration/). It enables you to take advantage of Azure App Configuration for the centralized storage and management of your configuration without direct dependency on App Configuration by your applications.

## Installation

Use helm to install Azure App Configuration Kubernetes Provider.
``` bash
helm install azureappconfiguration.kubernetesprovider \
     oci://mcr.microsoft.com/azure-app-configuration/helmchart/kubernetes-provider \
     --namespace azappconfig-system \
     --create-namespace
```

## Getting started

Documentation on how to use the Azure App Configuration Kubernetes Provider is available in the following links:

+ [Use Azure App Configuration in Azure Kubernetes Service](https://learn.microsoft.com/azure/azure-app-configuration/quickstart-azure-kubernetes-service)
+ [Use dynamic configuration in Azure Kubernetes Service](https://learn.microsoft.com/azure/azure-app-configuration/enable-dynamic-configuration-azure-kubernetes-service)
+ See [Kubernetes Provider Reference](https://learn.microsoft.com/azure/azure-app-configuration/reference-kubernetes-provider) for a complete list of features.

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft 
trademarks or logos is subject to and must follow 
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies. 




