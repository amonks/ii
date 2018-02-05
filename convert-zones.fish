cd zones
for file in *
	if test -s $file
		../bzfttr53rdutil -domain $file -zone-file $file > ../modules/converted-zones/$file.tf
	else
		if test -s ../modules/sources/$file.tf
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
