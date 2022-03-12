# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure("2") do |config|
  # The most common configuration options are documented and commented below.
  # For a complete reference, please see the online documentation at
  # https://docs.vagrantup.com.

  # Every Vagrant development environment requires a box. You can search for
  # boxes at https://vagrantcloud.com/search.
  config.vm.box = "ubuntu/focal64"

  # Disable automatic box update checking. If you disable this, then
  # boxes will only be checked for updates when the user runs
  # `vagrant box outdated`. This is not recommended.
  # config.vm.box_check_update = false

  config.vm.network "forwarded_port", guest: 22, host: 8022
  config.vm.network "forwarded_port", guest: 80, host: 8001

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine and only allow access
  # via 127.0.0.1 to disable public access
  # config.vm.network "forwarded_port", guest: 80, host: 8080, host_ip: "127.0.0.1"

  # Create a private network, which allows host-only access to the machine
  # using a specific IP.
  # config.vm.network "private_network", ip: "192.168.33.10"

  # Create a public network, which generally matched to bridged network.
  # Bridged networks make the machine appear as another physical device on
  # your network.
  # config.vm.network "public_network"

  # Share an additional folder to the guest VM. The first argument is
  # the path on the host to the actual folder. The second argument is
  # the path on the guest to mount the folder. And the optional third
  # argument is a set of non-required options.
  config.vm.synced_folder "./", "/vagrant"

  # Provider-specific configuration so you can fine-tune various
  # backing providers for Vagrant. These expose provider-specific options.
  # Example for VirtualBox:
  #
  # config.vm.provider "virtualbox" do |vb|
  #   # Display the VirtualBox GUI when booting the machine
  #   vb.gui = true
  #
  #   # Customize the amount of memory on the VM:
  #   vb.memory = "1024"
  # end
  #
  # View the documentation for the provider you are using for more
  # information on available options.

  # Here the VM is configured for developing the provider. This is similar to
  # how the CI pipeline is configured (see .circleci/config.yml). This vagrant
  # config isn't guaranteed to be as well maintained.
  config.vm.provision "shell", inline: <<-SHELL
    # Install dependencies
    apt update
    apt upgrade -y
    apt install -y gcc docker.io

    curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
    sudo apt-add-repository "deb [arch=$(dpkg --print-architecture)] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
    apt install -y terraform

    curl -OL https://golang.org/dl/go1.17.8.linux-amd64.tar.gz
    tar -C /usr/local -xvf go1.17.8.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/vagrant/.profile
    chown -R $(whoami): /usr/local/go
    
    curl -sSL https://github.com/gotestyourself/gotestsum/releases/download/v1.7.0/gotestsum_1.7.0_linux_amd64.tar.gz | \
    sudo tar -zx -C /usr/bin gotestsum

    # Pull some service images used by the acceptance test suite
    docker pull circleci/postgres:9.6.16-alpine-ram
    docker pull circleci/redis:6.2.5
    docker pull mysql:5.7.36

    # Run the dokku container
    docker container run \
      --env DOKKU_HOSTNAME=dokku.me \
      --name dokku \
      --publish 3022:22 \
      --publish 8080:80 \
      --publish 8443:443 \
      --volume /var/lib/dokku:/mnt/dokku \
      --volume /var/lib/dokku/services/:/var/lib/dokku/services/ \
      --volume /var/run/docker.sock:/var/run/docker.sock \
      -d \
      dokku/dokku:0.26.8

    # Configure an SSH key with which to authenticate with dokku
    ssh-keygen -t rsa -N "" -f dokku-ssh
    chown vagrant:vagrant dokku-ssh
    chown vagrant:vagrant dokku-ssh.pub
    docker cp dokku-ssh.pub dokku:/tmp/dokku-ssh.pub
    docker exec dokku dokku ssh-keys:add dokku dokku-ssh.pub

    # Install dokku plugins
    docker exec dokku sudo dokku plugin:install https://github.com/dokku/dokku-postgres.git postgres
    docker exec dokku sudo dokku plugin:install https://github.com/dokku/dokku-redis.git redis
    docker exec dokku sudo dokku plugin:install https://github.com/dokku/dokku-mysql.git mysql
    docker exec dokku sudo dokku plugin:install https://github.com/dokku/dokku-clickhouse.git clickhouse

    # Set dokku env vars for credentials to SSH into the dokku container
    # This allows us to e.g just run `make testacc` and it should just work
    echo 'export DOKKU_SSH_HOST=127.0.0.1' >> /home/vagrant/.profile
    echo 'export DOKKU_SSH_PORT=3022' >> /home/vagrant/.profile
    echo 'export DOKKU_SSH_USER=dokku' >> /home/vagrant/.profile
    echo 'export DOKKU_SSH_CERT=/home/vagrant/dokku-ssh' >> /home/vagrant/.profile
  SHELL
end
