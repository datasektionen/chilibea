job "chilibea" {
  type = "service"

  group "chilibea" {
    network {
      port "http" { }
    }

    service {
      name     = "chilibea"
      port     = "http"
      provider = "nomad"
      tags = [
        "traefik.enable=true",
        "traefik.http.routers.chilibea.rule=Host(`chili.metadorerna.se`)",
        "traefik.http.routers.chilibea.tls.certresolver=default",
      ]
    }

    task "chilibea" {
      driver = "docker"

      config {
        image = var.image_tag
        ports = ["http"]
      }

      template {
        data        = <<ENV
{{ with nomadVar "nomad/jobs/chilibea" }}
APP_SECRET={{ .app_secret }}
OIDC_SECRET={{ .oidc_secret }}
HIVE_TOKEN={{ .hive_api_key }}
DATABASE_URL=postgresql://bea:{{ .database_password }}@postgres.dsekt.internal:5432/bea
{{ end }}
PORT={{ env "NOMAD_PORT_http" }}
OIDC_PROVIDER=https://sso.datasektionen.se/op
SSO_URL=https://sso.datasektionen.se
OIDC_ID=bea
OIDC_REDIRECT_URL=https://chili.metadorerna.se/oidc/callback
HIVE_URL=https://hive.datasektionen.se/api/v1
ENV
        destination = "local/.env"
        env         = true
      }

      resources {
        memory = 120
      }
    }
  }
}

variable "image_tag" {
  type = string
  default = "ghcr.io/datasektionen/zaiko:latest"
}
