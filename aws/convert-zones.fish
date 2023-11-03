if test (uname -s) = "Darwin"
	set tfz53 ./tfz53-macos
end
if test (uname -s) = "Linux"
	set tfz53 ./tfz53-linux
end

rm terraform/generated_*.tf

for file in zones/*
	if test -s $file
		$tfz53 -domain (basename $file) -zone-file $file > terraform/generated_(basename $file).tf
	end
end
