if test -d terraform/converted-zones
	rm -r terraform/converted-zones
end
mkdir -p terraform/converted-zones

for file in zones/*
	if test -s $file
		./tfz53 -domain (basename $file) -zone-file $file > terraform/converted-zones/(basename $file).tf
	end
end

terraform fmt
