# frozen_string_literal: true

require 'json'
require 'securerandom'

namespace :errbit do
  desc 'Initializes users, applications and site configs for fleet-telemetry errbit'
  task init_errbit: :environment do
    if User.count.positive?
      p 'already initialized'
      next
    end

    Rake::Task['errbit:bootstrap'].invoke
    Rake::Task['db:mongoid:create_indexes'].invoke

    user_config = JSON.parse(File.read('/initialize/users.json'))
    apps_config = JSON.parse(File.read('/initialize/apps.json'))
    site_configs = JSON.parse(File.read('/initialize/site_configs.json'))

    User.destroy_all # Deleting default users created by errbit docker image
    User.create!(user_config['users']) # Adding custom users provided in the config
    SiteConfig.create!(site_configs)
    App.create!(apps_config['apps'])
  end
end
