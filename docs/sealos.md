# Deploying Glance on Sealos

[![Deploy on Sealos](https://sealos.io/Deploy-on-Sealos.svg)](https://sealos.io/products/app-store/glance)

Sealos provides a maintained one-click template for deploying Glance. The template defines the application resources, generated defaults, and storage settings in the Sealos templates repository at [`template/glance/index.yaml`](https://github.com/labring-actions/templates/blob/main/template/glance/index.yaml).

## Before deploying

- Review the generated template defaults and any user-provided inputs in Sealos.
- Review persistence and storage settings in the Sealos template before storing production data.
- Keep your Glance configuration backed up so it can be reused if you later move to another installation method.

## Deploy

1. Open the [Glance template on Sealos](https://sealos.io/products/app-store/glance).
2. Click **Deploy Now** and review the template settings.
3. Start the deployment and wait for Sealos to create the application resources.
4. Open the generated Sealos app URL.
5. Complete any first-run setup or login flow required by your Glance configuration.

## Updating configuration

Glance is configured through YAML files. After deploying, update your configuration from the Sealos application resources and restart the application if the changed settings require a reload.

For Glance configuration details, see [Configuring Glance](configuration.md).
