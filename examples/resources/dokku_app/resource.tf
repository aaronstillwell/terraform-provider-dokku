# Apps attached to the same network can reach each other via Docker DNS as
# http://<app>.web:<port>. With tld set, the alias becomes
# http://<app>.web.<tld>:<port>.
#
# Networks are created if they don't exist, and a deployed app is rebuilt when
# its network settings change. The provider never destroys networks, as other
# apps may still use them; remove them with `dokku network:destroy`.
resource "dokku_app" "geoip" {
  name = "geoip"

  networks {
    attach_post_create = "apps"
    tld                = "internal"
  }
}

resource "dokku_app" "api" {
  name = "api"

  networks {
    attach_post_create = "apps"
    tld                = "internal"
  }

  config_vars = {
    GEOIP_API_URL = "http://geoip.web.internal:8080"
  }
}
