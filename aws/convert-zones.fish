rm terraform/generated_*.tf

for file in zones/*
	if test -s $file
		go run ../cmd/tfz53 -domain (basename $file) -zone-file $file > terraform/generated_(basename $file).tf
	end
end
