html = File.read(ARGV[0])

out = html.gsub(/<h(\d)>(<a [^<]+>)?([^<]+)(<\/a>)?<\/h\d>/) do |m|
  n, content = $1, $3
  if n == "1"
    %Q{<h1><a href="">#{content}</a></h1>}.tap { |sub| puts sub }
  else
    name = content.downcase.gsub("_", "-").gsub(/["']/, "").gsub(/^\W+|\W+$/, "").gsub(/\W/, "-").gsub(/-+/, "-")
    %Q{<h#{n}><a name="#{name}" href="##{name}">#{content}</a></h#{n}>}.tap { |sub| puts sub }
  end
end

File.write(ARGV[0], out)