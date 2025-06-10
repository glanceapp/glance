This is a user-friendly Docker compose directory structure template for [Glance](https://github.com/glanceapp/glance) with most things configured and ready to use, such as the ability to add assets, custom CSS, environment variables, page includes, etc. The files can be found in the `root` directory of this repository.

## Usage

Create a new directory called `glance` as well as the template files within it by running:

```bash
mkdir glance && cd glance && curl -sL https://github.com/glanceapp/docker-compose-template/archive/refs/heads/main.tar.gz | tar -xzf - --strip-components 2
```

Then, edit the following files as desired:
* `docker-compose.yml` to configure the port, volumes and other containery things
* `config/home.yml` to configure the widgets or layout of the home page
* `config/glance.yml` if you want to change the theme or add more pages
* `.env` to configure environment variables that will be available inside configuration files
* `assets/user.css` to add custom CSS


When ready, run:

```bash
docker compose up -d
```

If you encounter any issues, you can check the logs by running:

```bash
docker compose logs
```
