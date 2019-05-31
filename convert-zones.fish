cd zones
for file in *
	if test -s $file
		../tfz53 -domain $file -zone-file $file > ../terraform/converted-zones/$file.tf
	else
		if test -s ../terraform/sources/$file.tf
			echo "deleting empty with source $file"
			# rm $file
		else
			echo "deleting empty without source $file"
			# rm $file
		end
	end
end

cd ..
terraform fmt
