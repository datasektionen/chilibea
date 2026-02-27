job "bea" {
  type = "service"

  group "bea" {
    network {
      port "http" { }
    }

    service {
      name     = "bea"
      port     = "http"
      provider = "nomad"
      tags = [
        "traefik.enable=true",
        "traefik.http.routers.bea.rule=Host(`metadorerna.se`)",
        "traefik.http.routers.bea.tls.certresolver=default",
      ]
    }

    task "bea" {
      driver = "docker"

      config {
        image = var.image_tag
        ports = ["http"]
      }

      template {
        data        = <<ENV
{{ with nomadVar "nomad/jobs/bea" }}
APP_SECRET={{ .app_secret }}
OIDC_SECRET={{ .oidc_secret }}
HIVE_TOKEN={{ .hive_api_key }}
DATABASE_URL=postgresql://bea:{{ .database_password }}@postgres.dsekt.internal:5432/bea
{{ end }}
PORT={{ env "NOMAD_PORT_http" }}
OIDC_PROVIDER=https://sso.datasektionen.se/op
SSO_URL=http://sso.nomad.dsekt.internal
OIDC_ID=bea
OIDC_REDIRECT_URL=https://metadorerna.se/oidc/callback
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
