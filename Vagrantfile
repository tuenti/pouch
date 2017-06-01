# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "coreos-stable"
  config.vm.box_url = "https://storage.googleapis.com/stable.release.core-os.net/amd64-usr/current/coreos_production_vagrant.json"

  config.vm.synced_folder ".", "/src"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "1024"
  end
end
